package runner

import (
	"testing"
	"time"
)

func TestSimpleShellCommands(t *testing.T) {
	testcases := []struct {
		command string
		want    string
	}{
		{"echo -n hello", "hello"},
		{"echo -n hello{world,foo}", "helloworld hellofoo"},
		{"seq 50 | grep -E 2$ | tail -1", "42\n"},
		{"seq 5", "1\n2\n3\n4\n5\n"},
	}
	for _, testcase := range testcases {
		res, err := Run(ShellCommand(testcase.command))
		if err != nil {
			t.Errorf("Run(ShellCommand(%q)) = err: %v", testcase.command, err)
			continue
		}
		if res.Stdout != testcase.want {
			t.Errorf("Run(ShellCommand(%q)) = %q want %q", testcase.command, res.Stdout, testcase.want)
		}
	}
}

func TestTiming(t *testing.T) {
	res, err := Run(ShellCommand("sleep 0.2s"))
	if err != nil {
		t.Fatalf("Run(ShellCommand(\"sleep 0.2s\") = err: %v", err)
	}
	runtime := res.Runtime()
	duration := runtime.Seconds()
	target := 0.2
	slack := 0.05
	if duration < (target-slack) || duration > (target+slack) {
		t.Errorf("Run(ShellCommand(\"sleep 0.2s\")) = took: %v want %v", runtime, target)
	}
}

func TestTimeout(t *testing.T) {
	_, err := Run(ShellCommand("sleep 0.2s"), WithTimeout(10*time.Millisecond))
	if err == nil {
		t.Errorf("Run(ShellCommand(\"sleep 0.2s\", WithTimeout(10*time.Millisecond)) = unexpected success")
	}
}

func TestFailure(t *testing.T) {
	_, err := Run(ShellCommand("exit 1"))
	if err == nil {
		t.Errorf("Run(ShellCommand(\"exit 1\")) = unexpected success")
	}
}
