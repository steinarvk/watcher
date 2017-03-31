package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/steinarvk/watcher/hostinfo"
	"github.com/steinarvk/watcher/runner"
	"github.com/steinarvk/watcher/secrets"
	"github.com/steinarvk/watcher/storage"
)

var (
	dbSecretsFilename = flag.String("db_secrets", "", "database secrets YAML file")
	nodePath          = flag.String("node_path", "", "path of node to import")
	timestampSeconds  = flag.Int64("timestamp_seconds", 0, "backdated timestamp (in seconds)")
)

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
	if *dbSecretsFilename == "" {
		return errors.New("missing required flag: --db_secrets")
	}

	if *timestampSeconds == 0 {
		return errors.New("missing required flag: --timestamp_seconds")
	}

	if *nodePath == "" {
		return errors.New("missing required flag: --node_path")
	}

	db, err := connectDB(*dbSecretsFilename)
	if err != nil {
		return err
	}

	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	info, err := hostinfo.Get()
	if err != nil {
		return err
	}

	t := time.Unix(*timestampSeconds, 0)
	result := &runner.Result{
		Start:   t,
		Stop:    t,
		Success: true,
		Stdout:  string(data),
	}

	id, err := db.InsertExecution(*nodePath, result, info, nil)
	if err != nil {
		return err
	}

	log.Printf("imported %d byte(s) of data as %q id=%d", len(data), *nodePath, id)

	return nil
}

func main() {
	flag.Parse()

	os.Unsetenv("PGPASSFILE")

	if err := mainCore(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}
