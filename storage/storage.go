package storage

import (
	"database/sql"
	"log"
	"time"

	"github.com/lib/pq"
	"github.com/steinarvk/watcher/hostinfo"
	"github.com/steinarvk/watcher/runner"

	"github.com/prometheus/client_golang/prometheus"
)

var Verbose = false

var (
	metricQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "sql_queries",
			Help:      "Number of SQL queries",
		},
		[]string{"query"},
	)

	metricQueriesFinished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "sql_queries_finished",
			Help:      "Number of SQL queries finished (by status)",
		},
		[]string{"query", "status"},
	)

	metricQueryLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "watcher",
			Name:      "sql_query_latency_summary",
			Help:      "Latency of SQL queries",
		},
		[]string{"query", "status"},
	)
)

func init() {
	prometheus.MustRegister(metricQueries)
	prometheus.MustRegister(metricQueriesFinished)
	prometheus.MustRegister(metricQueryLatency)
}

type queryTracker struct {
	name string
	t0   time.Time
}

func beginTracking(name string) *queryTracker {
	metricQueries.With(prometheus.Labels{
		"query": name,
	}).Inc()
	return &queryTracker{
		name: name,
		t0:   time.Now(),
	}
}

func (q *queryTracker) Finish(err error) error {
	t1 := time.Now()
	status := "ok"
	if err != nil {
		status = "error"
		if castErr, ok := err.(*pq.Error); ok {
			status = castErr.Code.Name()
		}
	}
	duration := t1.Sub(q.t0)
	durationSecs := duration.Seconds()

	labels := prometheus.Labels{
		"query":  q.name,
		"status": status,
	}

	metricQueriesFinished.With(labels).Inc()
	metricQueryLatency.With(labels).Observe(durationSecs)

	return err
}

type DB struct {
	DB *sql.DB
}

func toUTCMillis(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

func (d *DB) wrappedExec(name, sql string, args ...interface{}) (sql.Result, error) {
	track := beginTracking(name)
	result, err := d.DB.Exec(sql, args...)
	return result, track.Finish(err)
}

func (d *DB) InsertExecution(path string, result *runner.Result, info *hostinfo.HostInfo) error {
	if Verbose {
		log.Printf("InsertExecution(%q, ...)", path)
	}
	_, err := d.wrappedExec("insert-execution", `
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
	)
	return err
}

func (d *DB) CleanLeases(t time.Time) error {
	if Verbose {
		log.Printf("CleanLeases(%v)", t)
	}
	_, err := d.wrappedExec("clean-leases", `
		DELETE FROM work_leases WHERE leased_until_utcmillis < $1
	`, toUTCMillis(t))
	return err
}

type Lease struct {
	db  *DB
	key string
	id  int64
}

func (l *Lease) Release() error {
	if Verbose {
		log.Printf("Lease.Release(%q,%v)", l.key, l.id)
	}
	_, err := l.db.DB.Exec(`
		DELETE FROM work_leases
		WHERE lease_id = $1 AND lease_key = $2
	`, l.id, l.key)
	return err
}

func (d *DB) TryObtainLease(key string, deadline time.Time) (*Lease, error) {
	if Verbose {
		log.Printf("TryObtainLease(%q, %v)", key, deadline)
	}

	var leaseId int64

	track := beginTracking("try-obtain-lease")
	err := d.DB.QueryRow(`
		INSERT INTO work_leases
			(lease_key, leased_until_utcmillis)
				VALUES
		  ($1, $2)
		RETURNING lease_id
	`, key, toUTCMillis(deadline)).Scan(&leaseId)
	track.Finish(err)

	if err == nil {
		return &Lease{d, key, leaseId}, nil
	}

	if castErr, ok := err.(*pq.Error); ok {
		if castErr.Code.Name() == "unique_violation" {
			return nil, nil
		}
	}

	return nil, err
}

func (d *DB) Unschedule(path string) error {
	_, err := d.wrappedExec("unschedule", `
		DELETE FROM scheduling_queue WHERE node_path = $1
	`, path)
	return err
}

func (d *DB) ScheduleEvent(path string, t time.Time) error {
	if Verbose {
		log.Printf("ScheduleEvent(%q, %v)", path, t)
	}
	_, err := d.wrappedExec("schedule-event", `
		INSERT INTO scheduling_queue
			(node_path, target_time_utcmillis)
				VALUES
		  ($1, $2)
	`,
		path, toUTCMillis(t),
	)
	if castErr, ok := err.(*pq.Error); ok {
		if castErr.Code.Name() == "unique_violation" {
			return nil
		}
	}
	return err
}

func (d *DB) NextScheduledSpecificEvent(path string) (time.Time, bool, error) {
	if Verbose {
		log.Printf("NextScheduledSpecificEvent(%q)", path)
	}
	var millis int64

	track := beginTracking("next-scheduled-specific-event")
	err := d.DB.QueryRow(`
	  SELECT target_time_utcmillis
		FROM scheduling_queue
		WHERE node_path = $1
		ORDER BY target_time_utcmillis ASC
		LIMIT 1
	`, path).Scan(&millis)
	track.Finish(err)

	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	t := time.Unix(0, millis*int64(time.Millisecond))
	return t, true, nil
}

func (d *DB) WithLease(key string, dur time.Duration, callback func() error) error {
	if Verbose {
		log.Printf("WithLease(%q, %v)", key, dur)
	}
	now := time.Now()
	deadline := now.Add(dur)
	lease, err := d.TryObtainLease(key, deadline)
	if err != nil {
		return err
	}
	if lease == nil {
		return nil
	}
	defer func() {
		err := lease.Release()
		if err != nil {
			log.Printf("error: failed to release lease %v: %v", lease, err)
		}
	}()

	return callback()
}
