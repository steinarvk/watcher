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
				next := scheduleSpec.ScheduleNext(time.Now())

				log.Printf("%q scheduled for %v", watch.Name, next)

				scheduler.WaitUntil(next)

				log.Printf("running %q", watch.Name)

				result, err := runner.Run(runSpec, runner.WithTimeout(timeout))
				if err != nil {
					log.Printf("error: running %q: %v", watch.Name, err)
					continue
				}

				if err := db.InsertExecution(watch.Name, result, info); err != nil {
					log.Printf("error: storing result of %q: %v", watch.Name, err)
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
