package hostinfo

import (
	"fmt"
	"os"
	"time"
)

type HostInfo struct {
	Hostname string
	Time     time.Time
	Pid      int
}

func Get() (*HostInfo, error) {
	name, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("unable to get hostname: %v", err)
	}

	return &HostInfo{
		Hostname: name,
		Time:     time.Now(),
		Pid:      os.Getpid(),
	}, nil
}
