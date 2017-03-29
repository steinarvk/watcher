package config

import (
	"errors"
	"fmt"

	"github.com/steinarvk/watcher/runner"
)

type Config struct {
	Watch []*WatchSpec `yaml:"watch"`
}

func (c *Config) Check() error {
	for i, w := range c.Watch {
		if err := w.Check(); err != nil {
			return fmt.Errorf("in watch spec %d: %v", i, err)
		}
	}
	return nil
}

type AnalysisSpec struct {
	Name     string          `yaml:"name"`
	Run      *runner.Config  `yaml:"run"`
	Children []*AnalysisSpec `yaml:"analyse"`
}

func (c *AnalysisSpec) Check() error {
	if c.Name == "" {
		return errors.New("missing 'name'")
	}
	if c.Run == nil {
		return errors.New("missing 'run'")
	}
	for i, child := range c.Children {
		if err := child.Check(); err != nil {
			return fmt.Errorf("in analysis %d (%q): %v", i, child.Name, err)
		}
	}
	return nil
}

type WatchSpec struct {
	Name     string          `yaml:"name"`
	Run      *runner.Config  `yaml:"run"`
	Children []*AnalysisSpec `yaml:"analyse"`

	// Schedule *scheduler.Config `yaml:"schedule"`
}

func (c *WatchSpec) Check() error {
	if c.Name == "" {
		return errors.New("missing 'name'")
	}
	if c.Run == nil {
		return errors.New("missing 'run'")
	}
	if err := c.Run.Check(); err != nil {
		return fmt.Errorf("in run section: %v", err)
	}
	for i, child := range c.Children {
		if err := child.Check(); err != nil {
			return fmt.Errorf("in analysis %d (%q): %v", i, child.Name, err)
		}
	}
	return nil
}
