package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/steinarvk/watcher/config"
	"github.com/steinarvk/watcher/runner"
	yaml "gopkg.in/yaml.v2"
)

var (
	configFilename = flag.String("config", "", "config YAML file")
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

func mainCore() error {
	if *configFilename == "" {
		return errors.New("missing required flag: --config")
	}

	cfg, err := loadConfig(*configFilename)
	if err != nil {
		return err
	}

	for _, watch := range cfg.Watch {
		fmt.Println("executing", watch.Name)
		spec, err := watch.Run.ToSpec()
		if err != nil {
			return err
		}

		result, err := runner.Run(spec, runner.WithTimeout(5*time.Second))
		if err != nil {
			fmt.Println("error", err)
		} else {
			fmt.Println("success")
			fmt.Println("stdout:", result.Stdout)
			fmt.Println("stderr:", result.Stderr)
			fmt.Println("runtime:", result.Runtime())
		}
	}

	return nil
}

func main() {
	flag.Parse()
	if err := mainCore(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}
