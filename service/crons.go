// @Version : 1.0
// @Author  : steven.wong
// @Email   : 'wangxk1991@gamil.com'
// @Time    : 2024/01/19 15:32:15
// Desc     : scheduler service
package service

import (
	"context"
	"reflect"

	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
)

type Scheduler struct {
	ctx  context.Context
	jobs map[string]*Job
	cron *cron.Cron
}

func NewScheduler(ctx context.Context) *Scheduler {
	return &Scheduler{
		ctx:  ctx,
		jobs: make(map[string]*Job),
		cron: cron.New(),
	}
}

func (s *Scheduler) AddJob(job *Job) {
	if _, ok := s.jobs[job.Name]; ok {
		logrus.Errorf("scheduler %s already exists", job.Name)
		panic("scheduler " + job.Name + " already exists")
	}
	if job == nil {
		panic("job is nil")
	}
	if reflect.ValueOf(job.Runner).Kind() != reflect.Ptr {
		panic("job must be a pointer")
	}
	s.jobs[job.Name] = job
}

func (s *Scheduler) Start() {
	for _, job := range s.jobs {
		if err := s.cron.AddFunc(job.Cron, callJob(job)); err != nil {
			logrus.Errorf("add job %s failed, because: %+v", job.Name, err)
		}
	}
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
	logrus.Infof("scheduler service shutdown")
}

func callJob(job *Job) func() {
	j := reflect.ValueOf(job.Runner).MethodByName(job.Method).Interface()
	if sampleFunc, ok := j.(func()); ok {
		return sampleFunc
	} else {
		panic("Conversion scheduler failed.")
	}
}

type JobStatus string

var (
	JobStatusInit    JobStatus = "init"
	JobStatusRunning JobStatus = "running"
	JobStatusDone    JobStatus = "done"
	JobStatusError   JobStatus = "error"
)

type Job struct {
	Name   string
	Cron   string
	Method string
	Runner any
}
