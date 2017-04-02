package runner

type Python3Command string

const (
	python3Name = "python3"

	standardPythonPrelude = "import sys, json;"
)

func (s Python3Command) Program() string { return python3Name }
func (s Python3Command) Args() []string {
	return []string{"-c", standardPythonPrelude + string(s)}
}
