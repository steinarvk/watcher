package scheduler

import (
	"errors"
	"fmt"
	"time"
)

type RandomConfig struct {
	Min string `yaml:"min"`
	Max string `yaml:"max"`
}

type Config struct {
	Period string        `yaml:"period"`
	Random *RandomConfig `yaml:"random"`
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

func (c *Config) ToSpec() (Scheduler, error) {
	n := countTrue(
		c.Period != "",
		c.Random != nil,
	)
	if n == 0 {
		return nil, errors.New("empty scheduler config")
	}
	if n > 1 {
		return nil, fmt.Errorf("ambiguous scheduler config: %v", c)
	}

	switch {
	case c.Period != "":
		dur, err := parseDuration(c.Period)
		if err != nil {
			return nil, fmt.Errorf("invalid 'period' %q: %v", c.Period, err)
		}

		return Periodic(dur), nil

	case c.Random != nil:
		minDur, err := parseDuration(c.Random.Min)
		if err != nil {
			return nil, fmt.Errorf("invalid 'random.min' %q: %v", c.Random.Min, err)
		}

		maxDur, err := parseDuration(c.Random.Max)
		if err != nil {
			return nil, fmt.Errorf("invalid 'random.min' %q: %v", c.Random.Max, err)
		}

		if maxDur < minDur {
			return nil, fmt.Errorf("invalid random scheduler: random.max < random.min (%v < %v)", maxDur, minDur)
		}

		return UniformRandom{minDur, maxDur}, nil

	default:
		return nil, fmt.Errorf("internal error handling scheduler config: %v", c)
	}
}

func (c *Config) Check() error {
	_, err := c.ToSpec()
	return err
}

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, errors.New("not a valid duration: empty string")
	}
	return time.ParseDuration(s)
}
