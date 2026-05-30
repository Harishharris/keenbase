package keenbase

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

type Job struct {
	ID       string
	Name     string
	Schedule string
	f        func()
}

type Cron struct {
	mu         sync.Mutex
	startTime  *time.Timer
	ticker     *time.Ticker
	duration   time.Duration
	tickerDone chan bool
	jobs       []*Job
}

func NewCron() *Cron {
	return &Cron{
		jobs:       []*Job{},
		duration:   1 * time.Second,
		tickerDone: make(chan bool),
	}
}

func (c *Cron) Add(name string, schedule string, f func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	job := &Job{
		ID:       randomBase64(16),
		Name:     name,
		Schedule: schedule, // this is not being used currently, but it can be used in the future to support cron expressions for scheduling jobs at specific times or intervals.
		f:        f,
	}
	c.jobs = append(c.jobs, job)
}

func (c *Cron) SetInterval(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.duration = duration
}

func (c *Cron) Cron() []Job {
	c.mu.Lock()
	defer c.mu.Unlock()
	jobs := []Job{}

	for _, job := range c.jobs {
		jobs = append(jobs, *job)
	}
	return jobs
}

func (c *Cron) Start() {
	c.Stop()

	if c.duration == 0 {
		panic("cron: cron job intervall cannot be 0, set it using SetInterval(seconds int)")
	}

	fmt.Println("starting cron with interval", c.duration)
	now := time.Now()
	next := now.Add(c.duration).Truncate(c.duration)
	delay := next.Sub(now)

	c.startTime = time.AfterFunc(delay, func() {
		c.ticker = time.NewTicker(c.duration)
		go func() {
			for {
				select {
				case <-c.tickerDone:
					return
				case <-c.ticker.C:
					for _, job := range c.jobs {
						go job.f()
					}
				}
			}
		}()
	})
}

func (c *Cron) Stop() {
	if c.startTime != nil {
		c.startTime.Stop()
		c.startTime = nil
	}
	if c.ticker == nil {
		return
	}
	c.tickerDone <- true
	c.ticker.Stop()
	c.ticker = nil
}

func randomBase64(n int) string {
	b := make([]byte, n)

	_, err := rand.Read(b)
	if err != nil {
		return ""
	}

	return base64.RawURLEncoding.EncodeToString(b)
}
