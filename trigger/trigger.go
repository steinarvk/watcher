package trigger

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/steinarvk/watcher/config"
	"github.com/steinarvk/watcher/hostinfo"
	"github.com/steinarvk/watcher/runner"
	"github.com/steinarvk/watcher/scheduler"
	"github.com/steinarvk/watcher/storage"
)

var (
	Verbose = false
)

type cmdTracker struct {
	name string
	t0   time.Time
}

func beginTracking(name string) *cmdTracker {
	metricTriggerRuns.With(prometheus.Labels{
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

	metricTriggerRunsFinished.With(labels).Inc()
	metricTriggerRunLatency.With(labels).Observe(durationSecs)

	return err
}

var (
	metricTriggersStarted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "triggers_started",
			Help:      "Trigger workers that have been started",
		},
		[]string{"trigger"},
	)

	metricNodeStoredHintsReceived = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "trigger_node_stored_hints_received",
			Help:      "Number of times we've received a 'node stored' hint from a channel",
		},
	)

	metricTriggerRuns = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "trigger_commands",
			Help:      "Number of trigger commands run",
		},
		[]string{"name"},
	)

	metricTriggerRunsFinished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "trigger_commands_finished",
			Help:      "Number of trigger commands finished (by status)",
		},
		[]string{"name", "status"},
	)

	metricTriggerRunLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "watcher",
			Name:      "trigger_commands_latency_summary",
			Help:      "Latency of trigger commands",
		},
		[]string{"name", "status"},
	)
)

func init() {
	prometheus.MustRegister(metricTriggersStarted)
	prometheus.MustRegister(metricNodeStoredHintsReceived)
	prometheus.MustRegister(metricTriggerRuns)
	prometheus.MustRegister(metricTriggerRunsFinished)
	prometheus.MustRegister(metricTriggerRunLatency)
}

func TriggerWorker(db *storage.DB, parentPath, path string, spec *config.TriggerSpec, notify <-chan struct{}, nodesStored chan<- string) error {
	log.Printf("starting trigger-worker for node %q", path)

	metricTriggersStarted.WithLabelValues(path).Inc()

	info, err := hostinfo.Get()
	if err != nil {
		return fmt.Errorf("error getting hostinfo: %v", err)
	}

	triggerPeriod, err := time.ParseDuration(spec.Period)
	if err != nil {
		return err
	}

	runSpec, err := spec.Run.ToSpec()
	if err != nil {
		return err
	}

	if !runSpec.ShouldRun() {
		return errors.New("do-not-run for trigger makes no sense")
	}

	runTimeout, err := spec.Run.GetTimeout()
	if err != nil {
		return err
	}

	checkScheduler := scheduler.UniformRandom{
		time.Minute,
		2 * time.Minute,
	}

	timeout := make(chan struct{}, 100)
	go func() {
		for {
			scheduler.WaitUntil(checkScheduler.ScheduleNext(time.Now()))
			timeout <- struct{}{}
		}
	}()

	for {
		select {
		case <-notify:
			metricNodeStoredHintsReceived.Inc()
			if Verbose {
				log.Printf("trigger-worker %q woke up: notified", path)
			}
		case <-timeout:
			if Verbose {
				log.Printf("trigger-worker %q woke up: timeout", path)
			}
		}

		item, err := db.GetLatestExecutionIfChildless(parentPath, path)
		if err != nil {
			return err
		}
		if item == nil {
			continue
		}
		triggerInput := strings.TrimSpace(item.Result.Stdout)
		if triggerInput == "" {
			continue
		}

		if Verbose {
			log.Printf("would trigger %q: %q [item root time %q]", path, triggerInput, item.RootTime)
		}

		lastTrigger, err := db.GetTimeOfLatestSuccessfulExecution(path)
		if err != nil {
			return err
		}

		if lastTrigger != nil {
			if dur := time.Since(*lastTrigger); dur < triggerPeriod {
				log.Printf("skipping trigger %q: only %v since last trigger (%v, period %v)", path, dur, *lastTrigger, triggerPeriod)
				continue
			}
		}

		err = db.WithLease(fmt.Sprintf("trigger:%s:%d", path, item.Id), runTimeout+time.Second, func() error {
			log.Printf("running trigger %q: [root time: %v] %q", path, item.RootTime, triggerInput)

			track := beginTracking(path)
			result, err := runner.Run(runSpec, runner.WithTimeout(runTimeout), runner.WithInput(triggerInput))
			track.Finish(err)

			// An error running the command is not actually a trigger error.
			// We still store the result.
			if err != nil {
				log.Printf("error running trigger %q (%d): %v", path, item.Id, err)
			} else {
				if Verbose {
					log.Printf("ran trigger %q (ok)", path)
				}
			}

			_, err = db.InsertExecution(path, result, info, &item.Id)
			nodesStored <- path
			return err
		})
		if err != nil {
			return err
		}
	}
}
