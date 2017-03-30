package storage

import (
	"database/sql"
	"time"

	"github.com/steinarvk/watcher/hostinfo"
	"github.com/steinarvk/watcher/runner"
)

type DB struct {
	DB *sql.DB
}

func toUTCMillis(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

func (d *DB) InsertExecution(path string, result *runner.Result, info *hostinfo.HostInfo) error {
	return d.DB.QueryRow(`
		INSERT INTO program_executions
			(node_path,
		   executor_host, executor_pid,
			 started_utcmillis, stopped_utcmillis,
			 success,
			 stdout, stderr)
			VALUES
			($1,
			 $2, $3,
			 $4, $5,
			 $6,
			 $7, $8)
	`,
		path,
		info.Hostname, info.Pid,
		toUTCMillis(result.Start), toUTCMillis(result.Stop),
		result.Success,
		result.Stdout, result.Stderr,
	).Scan()
}
