package analyse

import (
	"errors"
	"fmt"
	"log"
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
	metricAnalyseRuns.With(prometheus.Labels{
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

	metricAnalyseRunsFinished.With(labels).Inc()
	metricAnalyseRunLatency.With(labels).Observe(durationSecs)

	return err
}

var (
	metricAnalysersStarted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "analysers_started",
			Help:      "Analysers that have been started",
		},
		[]string{"analyser"},
	)

	metricNodeStoredHintsReceived = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "node_stored_hints_received",
			Help:      "Number of times we've received a 'node stored' hint from a channel",
		},
	)

	metricAnalyseRuns = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "analyse_commands",
			Help:      "Number of analyse commands run",
		},
		[]string{"name"},
	)

	metricAnalyseRunsFinished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "watcher",
			Name:      "analyse_commands_finished",
			Help:      "Number of analyse commands finished (by status)",
		},
		[]string{"name", "status"},
	)

	metricAnalyseRunLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "watcher",
			Name:      "analyse_commands_latency_summary",
			Help:      "Latency of analyse commands",
		},
		[]string{"name", "status"},
	)
)

func init() {
	prometheus.MustRegister(metricAnalysersStarted)
	prometheus.MustRegister(metricNodeStoredHintsReceived)
	prometheus.MustRegister(metricAnalyseRuns)
	prometheus.MustRegister(metricAnalyseRunsFinished)
	prometheus.MustRegister(metricAnalyseRunLatency)
}

func Analyse(db *storage.DB, parentPath, path string, spec *config.AnalysisSpec, notify <-chan struct{}, nodesStored chan<- string) error {
	log.Printf("starting analyser for node %q", path)

	metricAnalysersStarted.WithLabelValues(path).Inc()

	info, err := hostinfo.Get()
	if err != nil {
		return fmt.Errorf("error getting hostinfo: %v", err)
	}

	runSpec, err := spec.Run.ToSpec()
	if err != nil {
		return err
	}

	if !runSpec.ShouldRun() {
		return errors.New("do-not-run for analyser makes no sense")
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

	skipDelay := true

	for {
		if !skipDelay {
			select {
			case <-notify:
				metricNodeStoredHintsReceived.Inc()
				if Verbose {
					log.Printf("analyser %q woke up: notified", path)
				}
			case <-timeout:
				if Verbose {
					log.Printf("analyser %q woke up: timeout", path)
				}
			}
		}

		items, more, err := db.GetChildlessExecutions(parentPath, path)
		if err != nil {
			return err
		}
		skipDelay = more

		for _, item := range items {
			err := db.WithLease(fmt.Sprintf("analyse:%s:%d", path, item.Id), runTimeout+time.Second, func() error {
				log.Printf("running analysis %q", path)

				track := beginTracking(path)
				result, err := runner.Run(runSpec, runner.WithTimeout(runTimeout), runner.WithInput(item.Stdout))
				track.Finish(err)

				// An error running the command is not actually an analysis error.
				// We still store the result.
				if err != nil {
					log.Printf("error running analyse %q (%d): %v", path, item.Id, err)
				} else {
					if Verbose {
						log.Printf("ran analyse %q (ok)", path)
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
}
