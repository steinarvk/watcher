package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/steinarvk/watcher/config"
	"github.com/steinarvk/watcher/hostinfo"
	"github.com/steinarvk/watcher/runner"
	"github.com/steinarvk/watcher/scheduler"
	"github.com/steinarvk/watcher/secrets"
	"github.com/steinarvk/watcher/storage"

	_ "github.com/lib/pq"

	yaml "gopkg.in/yaml.v2"
)

var (
	configFilename    = flag.String("config", "", "config YAML file")
	dbSecretsFilename = flag.String("db_secrets", "", "database secrets YAML file")
	verboseLogging    = flag.Bool("verbose", false, "verbose logging")
)

func loadConfig(filename string) (*config.Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading %q: %v", filename, err)
	}

	cfg := &config.Config{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error parsing %q: %v", filename, err)
	}

	if err := cfg.Check(); err != nil {
		return nil, fmt.Errorf("invalid config %q: %v", filename, err)
	}

	return cfg, nil
}

type DatabaseSecrets struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

func connectDB(secretsFilename string) (*storage.DB, error) {
	dbSecrets := DatabaseSecrets{}
	if err := secrets.FromYAML(secretsFilename, &dbSecrets); err != nil {
		return nil, err
	}

	opts := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=require", dbSecrets.Host, dbSecrets.Port, dbSecrets.Database, dbSecrets.User, dbSecrets.Password)

	dbSecrets.Password = ""
	sanitizedDBCreds := dbSecrets

	db, err := sql.Open("postgres", opts)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to DB (credentials: %v): %v", sanitizedDBCreds, err)
	}

	opts = ""

	return &storage.DB{db}, nil
}

func mainCore() error {
	if *verboseLogging {
		storage.Verbose = true
	}

	if *configFilename == "" {
		return errors.New("missing required flag: --config")
	}

	if *dbSecretsFilename == "" {
		return errors.New("missing required flag: --db_secrets")
	}

	db, err := connectDB(*dbSecretsFilename)
	if err != nil {
		return err
	}

	cfg, err := loadConfig(*configFilename)
	if err != nil {
		return err
	}

	info, err := hostinfo.Get()
	if err != nil {
		return fmt.Errorf("error getting hostinfo: %v", err)
	}

	go func() {
		for {
			if err := db.CleanLeases(time.Now()); err != nil {
				log.Fatalf("error: CleanLeases() = %v", err)
			}
			time.Sleep(10 * time.Second)
		}
	}()

	for _, watch := range cfg.Watch {
		watch := watch

		runSpec, err := watch.Run.ToSpec()
		if err != nil {
			return err
		}

		scheduleSpec, err := watch.Schedule.ToSpec()
		if err != nil {
			return err
		}

		timeout := 5 * time.Second

		go func() {
			for {
				next, got, err := db.NextScheduledSpecificEvent(watch.Name)
				if err != nil {
					log.Fatal(err)
				}
				if !got {
					err := db.WithLease("schedule:"+watch.Name, time.Second, func() error {
						next = scheduleSpec.ScheduleNext(time.Now())
						log.Printf("scheduling %q for %v", watch.Name, next)
						return db.ScheduleEvent(watch.Name, next)
					})
					if err != nil {
						log.Fatal(err)
					}
				}

				if next.IsZero() {
					log.Printf("no time scheduled for %q", watch.Name)
					time.Sleep(time.Second)
					continue
				}

				log.Printf("%q scheduled for %v", watch.Name, next)
				scheduler.WaitUntil(next)

				err = db.WithLease("execute:"+watch.Name, timeout+time.Second, func() error {
					if err := db.Unschedule(watch.Name); err != nil {
						return err
					}

					log.Printf("running %q", watch.Name)
					result, err := runner.Run(runSpec, runner.WithTimeout(timeout))
					if err != nil {
						return err
					}

					if err := db.InsertExecution(watch.Name, result, info); err != nil {
						return err
					}

					return nil
				})
				if err != nil {
					log.Fatal(err)
				}
			}
		}()
	}

	for {
		time.Sleep(time.Hour)
	}
}

func main() {
	flag.Parse()
	if err := mainCore(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}
