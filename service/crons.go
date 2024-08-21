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
	// schedulers []types.Scheduler
}

func NewScheduler(ctx context.Context) *Scheduler {
	return &Scheduler{
		ctx:  ctx,
		jobs: make(map[string]*Job),
		cron: cron.New(),
		// schedulers: make([]types.Scheduler, 0),
	}
}

// func (s *Scheduler) run() {
// 	c := cron.New()
// 	for _, job := range s.jobs {
// 		if err := c.AddFunc(job.Cron, s.callJob()); err != nil {
// 			logrus.Errorf("add job %s failed, because: %+v", job.Name, err)
// 		}
// 	}
// 	c.Start()
// defer c.Stop()
// }

// func (s *Scheduler) add(job any) {
// 	if _, ok := s.jobs[job.Name]; ok {
// 		logrus.Errorf("scheduler %s already exists", job.Name)
// 		panic("scheduler " + job.Name + " already exists")
// 	}
// 	s.jobs[job.Name] = job
// }

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

// func (s *Scheduler) Shutdown() error {
// 	for _, job := range s.jobs {
// 		job.Cancel()
// 	}
// 	return nil
// }

func (s *Scheduler) Start() {
	for _, job := range s.jobs {
		if err := s.cron.AddFunc(job.Cron, callJob(job)); err != nil {
			logrus.Errorf("add job %s failed, because: %+v", job.Name, err)
		}
	}
	s.cron.Start()
	// for _, scheduler := range s.schedulers {
	// 	if _, ok := s.jobs[scheduler.Name]; ok {
	// 		logrus.Errorf("scheduler %s already exists", scheduler.Name)
	// 		panic("scheduler " + scheduler.Name + " already exists")
	// 	}
	// 	if scheduler.Enabled {
	// 		ctx, cancel := context.WithCancel(s.ctx)
	// 		s.add(&Job{
	// 			Name:   scheduler.Name,
	// 			Detail: scheduler.Detail,
	// 			Cron:   scheduler.Cron,
	// 			Method: scheduler.Method,
	// 			Status: JobStatusInit,
	// 			Ctx:    ctx,
	// 			Cancel: cancel,
	// 			Args:   scheduler.Args,
	// 		})
	// 	}
	// }
	// s.run()

}

func (s *Scheduler) Stop() {
	// s.Shutdown(s.ctx)
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
	Name string
	// Detail string
	Cron   string
	Method string
	// Done   chan struct{}
	// Status JobStatus
	// Args   map[string]interface{}
	Runner any
	// params any
	// start  time.Time
}

// func (j *Job) Demo() {
// 	j.init(nil)
// 	defer j.end()
// 	logrus.Infof("job %s demo", j.Name)
// }

// func (j *Job) init(params any) {
// 	logrus.Printf("[job] %s: %s", j.Name, j.Detail)
// 	j.Status = JobStatusRunning
// 	j.params = params
// 	j.start = time.Now().Local()
// }

// func (j *Job) end() {
// 	j.Status = JobStatusDone
// 	logrus.Infof("[job] %s done, cost time: %s", j.Name, time.Now().Local().Sub(j.start).String())
// }
