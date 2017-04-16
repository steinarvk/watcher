package runner

import (
	"errors"
	"fmt"
	"time"
)

var (
	DefaultTimeout = 5 * time.Second
)

type ProgramSpec struct {
	Binary    string   `yaml:"binary"`
	Arguments []string `yaml:"args"`
}

func (p *ProgramSpec) Program() string { return p.Binary }
func (p *ProgramSpec) Args() []string  { return p.Arguments }
func (p *ProgramSpec) ShouldRun() bool { return true }

type DoNotRunSpec struct{}

func (p *DoNotRunSpec) Program() string { return "/bin/true" }
func (p *DoNotRunSpec) Args() []string  { return nil }
func (p *DoNotRunSpec) ShouldRun() bool { return false }

func whichFile(path string) (bool, error) {
	whichSpec := &ProgramSpec{
		Binary:    "which",
		Arguments: []string{path},
	}

	whichRes, err := Run(whichSpec, WithTimeout(time.Second))
	if err != nil {
		return false, err
	}

	return whichRes.Success, nil
}

type Config struct {
	Shell    string       `yaml:"shell"`
	Program  *ProgramSpec `yaml:"program"`
	Python3  string       `yaml:"python3"`
	DoNotRun bool         `yaml:"do-not-run"`

	Timeout string `yaml:"timeout"`
}

func (c *Config) GetTimeout() (time.Duration, error) {
	if c.Timeout == "" {
		return DefaultTimeout, nil
	}
	return time.ParseDuration(c.Timeout)
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
		c.Python3 != "",
		c.Program != nil,
		c.DoNotRun,
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

	case c.Python3 != "":
		return Python3Command(c.Python3), nil

	case c.Program != nil:
		return c.Program, nil

	case c.DoNotRun:
		return &DoNotRunSpec{}, nil

	default:
		return nil, fmt.Errorf("internal error handling runner config: %v", c)
	}
}

func (c *Config) Check() error {
	spec, err := c.ToSpec()
	if err != nil {
		return err
	}
	ok, err := whichFile(spec.Program())
	if err != nil || !ok {
		return fmt.Errorf("will be unable to execute command (%v): which(%q) = %v (err: %v)", c, spec.Program(), ok, err)
	}
	return nil
}
