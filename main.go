package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/steinarvk/watcher/analyse"
	"github.com/steinarvk/watcher/config"
	"github.com/steinarvk/watcher/secrets"
	"github.com/steinarvk/watcher/storage"
	"github.com/steinarvk/watcher/watch"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/lib/pq"

	yaml "gopkg.in/yaml.v2"
)

var (
	DefaultPort = 5365
)

var (
	configFilename    = flag.String("config", "", "config YAML file")
	dbSecretsFilename = flag.String("db_secrets", "", "database secrets YAML file")
	verboseLogging    = flag.Bool("verbose", false, "verbose logging")
	listenHost        = flag.String("listen_host", "localhost", "listen on all network interfaces, not only localhost")
	port              = flag.Int("port", 0, "port on which to listen")
)

var (
	metricNodeDataStored = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "node_data_stored",
			Help:      "Number of times data for a node was inserted into the database",
		},
		[]string{"path"},
	)

	metricNodeStoredHintsSent = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "node_stored_hints_sent",
			Help:      "Number of times we've sent a 'node stored' hint on a channel",
		},
	)
)

func init() {
	prometheus.MustRegister(metricNodeDataStored)
	prometheus.MustRegister(metricNodeStoredHintsSent)
}

func beginListening() (net.Listener, error) {
	host := "127.0.0.1"
	if *listenHost != "localhost" {
		host = *listenHost
	}

	if *port != 0 {
		return net.Listen("tcp", fmt.Sprintf("%s:%d", host, *port))
	}

	// Attempt the preferred port first.
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, DefaultPort))
	if err == nil {
		return listener, nil
	}

	return net.Listen("tcp", fmt.Sprintf("%s:%d", host, 0))
}

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
		watch.Verbose = true
		analyse.Verbose = true
	}

	if *configFilename == "" {
		return errors.New("missing required flag: --config")
	}

	if *dbSecretsFilename == "" {
		return errors.New("missing required flag: --db_secrets")
	}

	listener, err := beginListening()
	if err != nil {
		return err
	}
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("listening on: http://%s/metrics", listener.Addr())
	go func() {
		// Listen forever, unless something goes wrong.
		log.Fatal(http.Serve(listener, nil))
	}()

	db, err := connectDB(*dbSecretsFilename)
	if err != nil {
		return err
	}

	cfg, err := loadConfig(*configFilename)
	if err != nil {
		return err
	}

	go func() {
		for {
			if err := db.CleanLeases(time.Now()); err != nil {
				log.Fatalf("error: CleanLeases() = %v", err)
			}
			time.Sleep(10 * time.Second)
		}
	}()

	nodesStored := make(chan string, 100)

	analyserChans := map[string][]chan<- struct{}{}

	var startAnalyser func(string, *config.AnalysisSpec) error

	startAnalyser = func(parentPath string, analysisSpec *config.AnalysisSpec) error {
		notifyChan := make(chan struct{}, 100)

		analyserChans[parentPath] = append(analyserChans[parentPath], notifyChan)

		path := parentPath + "/" + analysisSpec.Name
		for _, ch := range analysisSpec.Children {
			if err := startAnalyser(path, ch); err != nil {
				return err
			}
		}

		go func() {
			err := analyse.Analyse(db, parentPath, path, analysisSpec, notifyChan, nodesStored)
			if err != nil {
				log.Fatal(fmt.Errorf("error with analyser %q: %v", path, err))
			}
		}()

		return nil
	}

	for _, w := range cfg.Watch {
		go func(w *config.WatchSpec) {
			err := watch.Watch(db, w, nodesStored)
			if err != nil {
				log.Fatal(fmt.Errorf("error with watcher %q: %v", w.Name, err))
			}
		}(w)
		for _, ch := range w.Children {
			if err := startAnalyser(w.Name, ch); err != nil {
				log.Fatal(err)
			}
		}
	}

	for path := range nodesStored {
		metricNodeDataStored.WithLabelValues(path).Inc()
		for _, ch := range analyserChans[path] {
			metricNodeStoredHintsSent.Inc()
			ch <- struct{}{}
		}
	}
	return errors.New("impossible: exhausted neverending channel (nodesStored)")
}

func main() {
	flag.Parse()

	os.Unsetenv("PGPASSFILE")

	if err := mainCore(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}
