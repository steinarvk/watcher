package runner

import (
	"errors"
	"fmt"
)

type ProgramSpec struct {
	Binary    string   `yaml:"binary"`
	Arguments []string `yaml:"args"`
}

func (p *ProgramSpec) Program() string { return p.Binary }
func (p *ProgramSpec) Args() []string  { return p.Arguments }

type Config struct {
	Shell   string       `yaml:"shell"`
	Program *ProgramSpec `yaml:"program"`
}

func countTrue(xs ...bool) int {
	var rv int
	for _, x := range xs {
		if x {
			rv++
		}
	}
	return rv
}

func (c *Config) ToSpec() (Spec, error) {
	n := countTrue(
		c.Shell != "",
		c.Program != nil,
	)
	if n == 0 {
		return nil, errors.New("empty runner config")
	}
	if n > 1 {
		return nil, fmt.Errorf("ambiguous runner config: %v", c)
	}

	switch {
	case c.Shell != "":
		return ShellCommand(c.Shell), nil

	case c.Program != nil:
		return c.Program, nil

	default:
		return nil, fmt.Errorf("internal error handling runner config: %v", c)
	}
}

func (c *Config) Check() error {
	_, err := c.ToSpec()
	return err
}
