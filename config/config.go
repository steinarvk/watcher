package config

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

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

func checkNodeName(s string) error {
	if s == "" {
		return errors.New("missing 'name'")
	}
	if strings.Contains(s, "/") {
		return fmt.Errorf("invalid name %q: cannot contain '/'", s)
	}
	initial, _ := utf8.DecodeRuneInString(s)
	if strings.ContainsRune("0123456789_", initial) {
		return fmt.Errorf("invalid name %q: first character cannot be %v", initial)
	}
	return nil
}

func (c *AnalysisSpec) Check() error {
	if err := checkNodeName(c.Name); err != nil {
		return err
	}
	if c.Run == nil {
		return errors.New("missing 'run'")
	}
	seen := map[string]bool{}
	for i, child := range c.Children {
		if seen[child.Name] {
			return fmt.Errorf("child %q occurs twice", child.Name)
		}
		if err := child.Check(); err != nil {
			return fmt.Errorf("in analysis %d (%q): %v", i, child.Name, err)
		}
		seen[child.Name] = true
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
	if err := checkNodeName(c.Name); err != nil {
		return err
	}
	if c.Run == nil {
		return errors.New("missing 'run'")
	}
	if err := c.Run.Check(); err != nil {
		return fmt.Errorf("in run section: %v", err)
	}
	seen := map[string]bool{}
	for i, child := range c.Children {
		if seen[child.Name] {
			return fmt.Errorf("child %q occurs twice", child.Name)
		}
		if err := child.Check(); err != nil {
			return fmt.Errorf("in analysis %d (%q): %v", i, child.Name, err)
		}
		seen[child.Name] = true
	}
	return nil
}
