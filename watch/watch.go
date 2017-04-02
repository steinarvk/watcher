package watch

import (
	"fmt"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/steinarvk/watcher/config"
	"github.com/steinarvk/watcher/hostinfo"
	"github.com/steinarvk/watcher/runner"
	"github.com/steinarvk/watcher/scheduler"
	"github.com/steinarvk/watcher/storage"

	"github.com/cenkalti/backoff"
)

var (
	Verbose = false

	metricWatchersStarted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "watchers_started",
			Help:      "Watchers that have been started",
		},
		[]string{"watcher"},
	)

	metricWatchNextScheduledRun = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "watcher",
			Name:      "watch_next_scheduled_run",
			Help:      "Timestamp (Unix seconds) of next scheduled run",
		},
		[]string{"name"},
	)

	metricWatchRuns = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "watch_commands",
			Help:      "Number of watch commands run",
		},
		[]string{"name"},
	)

	metricWatchRunsFinished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "watch_commands_finished",
			Help:      "Number of watch commands finished (by status)",
		},
		[]string{"name", "status"},
	)

	metricWatchRunLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "watcher",
			Name:      "watch_commands_latency_summary",
			Help:      "Latency of watch commands",
		},
		[]string{"name", "status"},
	)
)

func init() {
	prometheus.MustRegister(metricWatchNextScheduledRun)
	prometheus.MustRegister(metricWatchRuns)
	prometheus.MustRegister(metricWatchRunsFinished)
	prometheus.MustRegister(metricWatchRunLatency)
	prometheus.MustRegister(metricWatchersStarted)
}

type cmdTracker struct {
	name string
	t0   time.Time
}

func beginTracking(name string) *cmdTracker {
	metricWatchRuns.With(prometheus.Labels{
		"name": name,
	}).Inc()
	return &cmdTracker{
		name: name,
		t0:   time.Now(),
	}
}

func (q *cmdTracker) Finish(err error) error {
	t1 := time.Now()
	status := "ok"
	if err != nil {
		status = "error"
	}
	duration := t1.Sub(q.t0)
	durationSecs := duration.Seconds()

	labels := prometheus.Labels{
		"name":   q.name,
		"status": status,
	}

	metricWatchRunsFinished.With(labels).Inc()
	metricWatchRunLatency.With(labels).Observe(durationSecs)

	return err
}

const (
	timeoutSlack = time.Second
)

func Watch(db *storage.DB, watch *config.WatchSpec, nodesStored chan<- string) error {
	log.Printf("starting watcher for node %q", watch.Name)

	metricWatchersStarted.WithLabelValues(watch.Name).Inc()

	info, err := hostinfo.Get()
	if err != nil {
		return fmt.Errorf("error getting hostinfo: %v", err)
	}

	runSpec, err := watch.Run.ToSpec()
	if err != nil {
		return err
	}

	timeout, err := watch.Run.GetTimeout()
	if err != nil {
		return err
	}

	scheduleSpec, err := watch.Schedule.ToSpec()
	if err != nil {
		return err
	}

	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = 0
	backoff.MaxInterval = 24 * time.Hour

	for {
		next, got, err := db.NextScheduledSpecificEvent(watch.Name)
		if err != nil {
			return err
		}

		if !got {
			err := db.WithLease("schedule:"+watch.Name, time.Second, func() error {
				next = scheduleSpec.ScheduleNext(time.Now())
				if Verbose {
					log.Printf("scheduling %q for %v", watch.Name, next)
				}
				return db.ScheduleEvent(watch.Name, next)
			})
			if err != nil {
				return err
			}
		}

		if next.IsZero() {
			if Verbose {
				log.Printf("no time scheduled for %q", watch.Name)
			}
			time.Sleep(time.Second)
			continue
		}

		metricWatchNextScheduledRun.WithLabelValues(watch.Name).Set(float64(next.UnixNano()) / float64(time.Second))

		if Verbose {
			log.Printf("%q scheduled for %v", watch.Name, next)
		}
		scheduler.WaitUntil(next)

		err = db.WithLease("execute:"+watch.Name, timeout+timeoutSlack, func() error {
			if err := db.Unschedule(watch.Name); err != nil {
				return err
			}

			log.Printf("running %q", watch.Name)

			track := beginTracking(watch.Name)
			result, err := runner.Run(runSpec, runner.WithTimeout(timeout))
			track.Finish(err)

			if err != nil {
				log.Printf("running %q: failed: %v", watch.Name, err)
				dur := backoff.NextBackOff()
				log.Printf("running %q failed: sleeping %v to throttle failures", watch.Name, dur)
				time.Sleep(dur)
				return nil
			}
			backoff.Reset()

			if _, err := db.InsertExecution(watch.Name, result, info, nil); err != nil {
				return err
			}
			nodesStored <- watch.Name

			return nil
		})
		if err != nil {
			return err
		}
	}
}
