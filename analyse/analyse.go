package analyse

import (
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/steinarvk/watcher/config"
	"github.com/steinarvk/watcher/scheduler"
)

var (
	Verbose = false
)

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
)

func init() {
	prometheus.MustRegister(metricAnalysersStarted)
	prometheus.MustRegister(metricNodeStoredHintsReceived)
}

func Analyse(parentPath, path string, spec *config.AnalysisSpec, notify <-chan struct{}, nodesStored chan<- string) error {
	log.Printf("starting analyser for node %q", path)

	metricAnalysersStarted.WithLabelValues(path).Inc()

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
				log.Printf("analyser %q woke up: notified", path)
			}
		case <-timeout:
			if Verbose {
				log.Printf("analyser %q woke up: timeout", path)
			}
		}

		log.Printf("TODO perform analysis on %q and store children", path)
	}
}
