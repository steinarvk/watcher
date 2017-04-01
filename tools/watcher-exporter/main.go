package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/steinarvk/watcher/secrets"
	"github.com/steinarvk/watcher/storage"
)

var (
	dbSecretsFilename = flag.String("db_secrets", "", "database secrets YAML file")
	nodePath          = flag.String("node_path", "", "path of node to import")
	showFailures      = flag.Bool("show_failures", false, "show output of failed commands")
	changesOnly       = flag.Bool("changes_only", false, "only show changes")
	trimValues        = flag.Bool("trim_shown_values", true, "trim spaces from beginning and end of shown values")
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

	if *nodePath == "" {
		return errors.New("missing required flag: --node_path")
	}

	db, err := connectDB(*dbSecretsFilename)
	if err != nil {
		return err
	}

	rows, err := db.QueryExecutionResults(*nodePath)
	if err != nil {
		return err
	}

	var lastValue *string

	for _, row := range rows {
		if !*showFailures && !row.Result.Success {
			continue
		}

		showValue := row.Result.Stdout
		if *trimValues {
			showValue = strings.TrimSpace(showValue)
		}

		if strings.Contains(showValue, "\n") {
			return fmt.Errorf("value would contain newline as shown: %v", row.Result.Stdout)
		}

		if *changesOnly {
			if lastValue != nil && *lastValue == showValue {
				continue
			}
			lastValue = &showValue
		}

		ms := row.RootTime.UnixNano() / int64(time.Millisecond)

		fmt.Printf("%v\t%s\n", ms, showValue)
	}

	return nil
}

func main() {
	flag.Parse()

	os.Unsetenv("PGPASSFILE")

	if err := mainCore(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}
