package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/steinarvk/secrets"
	"github.com/steinarvk/watcher/hostinfo"
	"github.com/steinarvk/watcher/runner"
	"github.com/steinarvk/watcher/storage"
)

var (
	dbSecretsFilename = flag.String("db_secrets", "", "database secrets YAML file")
	listenAddress     = flag.String("listen_address", "127.0.0.1:5753", "address on which to listen")
)

func connectDB(secretsFilename string) (*storage.DB, error) {
	dbSecrets := secrets.Postgres{}
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

	db, err := connectDB(*dbSecretsFilename)
	if err != nil {
		return err
	}

	info, err := hostinfo.Get()
	if err != nil {
		return err
	}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "POST" {
			http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
			return
		}

		data, err := ioutil.ReadAll(req.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		chunk := struct {
			TimestampMillis float64     `json:"timestamp_utcmillis"`
			NodePath        string      `json:"node_path"`
			Value           interface{} `json:"value"`
		}{}
		if err := json.Unmarshal(data, &chunk); err != nil {
			http.Error(w, "JSON parse error", http.StatusBadRequest)
			return
		}

		valueData, err := json.Marshal(chunk.Value)
		if err != nil {
			http.Error(w, "JSON re-wrap error", http.StatusBadRequest)
			return
		}

		t := time.Unix(0, int64(chunk.TimestampMillis)*int64(time.Millisecond))
		result := &runner.Result{
			Start:   t,
			Stop:    t,
			Success: true,
			Stdout:  string(valueData),
		}

		if t.Before(time.Unix(1400000000, 0)) || t.After(time.Now().Add(time.Minute)) {
			http.Error(w, "timestamp_utcmillis out of range", http.StatusBadRequest)
			return
		}

		if chunk.NodePath == "" {
			http.Error(w, "node_path not present", http.StatusBadRequest)
			return
		}

		id, err := db.InsertExecution(chunk.NodePath, result, info, nil)
		if err != nil {
			http.Error(w, "failed to write to database", http.StatusInternalServerError)
			return
		}

		log.Printf("imported %d byte(s) of data as %q id=%d", len(data), chunk.NodePath, id)

		w.Write([]byte(fmt.Sprintf("imported %d byte(s) of data as %q id=%d", len(data), chunk.NodePath, id)))
	})

	log.Printf("listening on %q", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
	return nil
}

func main() {
	flag.Parse()

	os.Unsetenv("PGPASSFILE")

	if err := mainCore(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}
