package scheduler

import (
	"math/rand"
	"time"
)

type Scheduler interface {
	ScheduleNext(t time.Time) time.Time
}

type Periodic time.Duration

func (d Periodic) ScheduleNext(t0 time.Time) time.Time {
	return t0.Add(time.Duration(d))
}

type UniformRandom struct {
	Min time.Duration
	Max time.Duration
}

func (d UniformRandom) ScheduleNext(t0 time.Time) time.Time {
	mn := d.Min.Seconds()
	mx := d.Max.Seconds()
	span := mx - mn
	secs := rand.Float64()*span + mn
	dur := time.Duration(secs * float64(time.Second))
	return t0.Add(dur)
}

func WaitUntil(t time.Time) {
	for time.Now().Before(t) {
		time.Sleep(t.Sub(time.Now()))
	}
}
