package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

type Result struct {
	Start time.Time
	Stop  time.Time

	Stdout string
	Stderr string

	Success bool
}

func (r Result) Runtime() time.Duration {
	return r.Stop.Sub(r.Start)
}

type options struct {
	ctx     context.Context
	input   string
	timeout time.Duration
}

type Option func(*options) error

func WithTimeout(dt time.Duration) Option {
	return func(o *options) error {
		o.timeout = dt
		return nil
	}
}

func WithInput(s string) Option {
	return func(o *options) error {
		o.input = s
		return nil
	}
}

type Spec interface {
	Program() string
	Args() []string
	ShouldRun() bool
}

func Run(spec Spec, opts ...Option) (*Result, error) {
	o := options{
		ctx: context.Background(),
	}
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	if o.timeout > 0 {
		newCtx, cancel := context.WithTimeout(o.ctx, o.timeout)
		defer cancel()

		o.ctx = newCtx
	}

	cmd := exec.CommandContext(o.ctx, spec.Program(), spec.Args()...)
	if o.input != "" {
		inputBuf := bytes.NewBufferString(o.input)
		cmd.Stdin = inputBuf
	}

	stdoutBuf := bytes.NewBuffer(nil)
	cmd.Stdout = stdoutBuf

	stderrBuf := bytes.NewBuffer(nil)
	cmd.Stderr = stderrBuf

	t0 := time.Now()
	err := cmd.Run()
	t1 := time.Now()
	if err != nil {
		_, ok := err.(*exec.ExitError)
		if !ok {
			return nil, fmt.Errorf("I/O error running command: %v", err)
		}
	}

	return &Result{
		Start:   t0,
		Stop:    t1,
		Success: cmd.ProcessState.Success(),
		Stdout:  stdoutBuf.String(),
		Stderr:  stderrBuf.String(),
	}, err
}
