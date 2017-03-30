package secrets

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	yaml "gopkg.in/yaml.v2"
)

var (
	ValidBasenameRE = regexp.MustCompile(`.*[.]secret[.]yaml$`)
)

func FromYAML(filename string, target interface{}) error {
	path, err := filepath.Abs(filename)
	if err != nil {
		return fmt.Errorf("error normalizing %q: %v", filename, err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error opening %q: %v", path, err)
	}

	base := filepath.Base(path)
	if !ValidBasenameRE.MatchString(base) {
		return fmt.Errorf("invalid secrets filename %q: must match %v", base, ValidBasenameRE)
	}

	if stat.IsDir() {
		return fmt.Errorf("error opening %q: is directory", path)
	}

	if perm := stat.Mode().Perm(); (perm & 0077) != 0 {
		return fmt.Errorf("error opening %q: permissions are %03o (077 permissions are forbidden)", path, perm)
	}

	data, err := ioutil.ReadFile(path)
	defer func() {
		for i := range data {
			data[i] = 0
		}
	}()
	if err != nil {
		return fmt.Errorf("error reading %q: %v", path, err)
	}

	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("error reading %q: %v", path, err)
	}

	return nil
}
