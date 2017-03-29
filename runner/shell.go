package runner

type ShellCommand string

const (
	shellName = "/bin/bash"
)

func (s ShellCommand) Program() string { return shellName }
func (s ShellCommand) Args() []string  { return []string{"-c", string(s)} }
