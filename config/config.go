package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/steinarvk/watcher/runner"
	"github.com/steinarvk/watcher/scheduler"
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

// TriggerSpec specifies a trigger. A trigger runs a command if an analysis
// finishes with a success exit and a nonempty (after trimming) stdout,
// only if that analysis is the _latest_ analysis yet seen, and only if the
// trigger has not triggered within the last <period>. The primary application
// is to send notifications.
type TriggerSpec struct {
	Name   string         `yaml:"name"`
	Period string         `yaml:"period"`
	Run    *runner.Config `yaml:"run"`
}

type AnalysisSpec struct {
	Name     string          `yaml:"name"`
	Run      *runner.Config  `yaml:"run"`
	Children []*AnalysisSpec `yaml:"analyse"`
	Triggers []*TriggerSpec  `yaml:"triggers"`
}

func checkNodeName(s string) error {
	if s == "" {
		return errors.New("missing 'name'")
	}
	if strings.Contains(s, "/") {
		return fmt.Errorf("invalid name %q: cannot contain '/'", s)
	}
	if strings.Contains(s, ":") {
		return fmt.Errorf("invalid name %q: cannot contain ':'", s)
	}
	initial, _ := utf8.DecodeRuneInString(s)
	if strings.ContainsRune("0123456789_", initial) {
		return fmt.Errorf("invalid name %q: first character cannot be %v", initial)
	}
	return nil
}

func (c *TriggerSpec) Check() error {
	if err := checkNodeName(c.Name); err != nil {
		return err
	}

	if c.Period == "" {
		return errors.New("missing period")
	}

	_, err := time.ParseDuration(c.Period)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %v", c.Period, err)
	}

	if c.Run == nil {
		return errors.New("missing 'run'")
	}

	return c.Run.Check()
}

func (c *AnalysisSpec) Check() error {
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
	for i, child := range c.Triggers {
		if seen[child.Name] {
			return fmt.Errorf("child %q occurs twice", child.Name)
		}
		if err := child.Check(); err != nil {
			return fmt.Errorf("in trigger %d (%q): %v", i, child.Name, err)
		}
		seen[child.Name] = true
	}
	return nil
}

type WatchSpec struct {
	Name     string            `yaml:"name"`
	Run      *runner.Config    `yaml:"run"`
	Schedule *scheduler.Config `yaml:"schedule"`
	Children []*AnalysisSpec   `yaml:"analyse"`
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
	if c.Schedule == nil {
		return errors.New("missing 'schedule'")
	}
	if err := c.Schedule.Check(); err != nil {
		return fmt.Errorf("in schedule section: %v", err)
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
