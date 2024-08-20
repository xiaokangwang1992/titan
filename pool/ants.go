/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2024/08/07 21:05:34
 Desc     :
*/

package pool

import (
	"context"
	"fmt"

	"github.com/panjf2000/ants/v2"
	"github.com/sirupsen/logrus"
)

type task struct {
	task func()
	opts *SubmitOptions
}

type pool struct {
	ctx  context.Context
	pool *ants.Pool
	task chan func()
}

type SubmitOptions struct {
	Timedout int64
}

func handlepanic(i interface{}) {

}

var apool *pool

func NewPool(ctx context.Context, size int, aopts *ants.Options) (*pool, error) {
	if apool != nil {
		return apool, nil
	}
	if aopts == nil {
		aopts = &ants.Options{
			ExpiryDuration:   60,
			Nonblocking:      false,
			PreAlloc:         true,
			MaxBlockingTasks: 10,
		}
	}
	p, err := ants.NewPool(size, func(opts *ants.Options) {
		opts.ExpiryDuration = aopts.ExpiryDuration
		opts.Nonblocking = aopts.Nonblocking
		opts.PreAlloc = aopts.PreAlloc
		opts.MaxBlockingTasks = aopts.MaxBlockingTasks
		opts.PanicHandler = handlepanic
	})
	if err != nil {
		return nil, err
	}
	apool = &pool{
		pool: p,
		ctx:  ctx,
		task: make(chan func(), 2),
	}
	return apool, nil
}

func (p *pool) Start() error {
	if p == nil {
		return fmt.Errorf("pool is nil")
	}
	for {
		select {
		case <-p.ctx.Done():
			p.pool.Release()
		case f := <-p.task:
			if err := p.pool.Submit(f); err != nil {
				logrus.Errorf("submit task failed, because: %+v", err)
			}
		}
	}
}

func (p *pool) Submit(f func(), opts *SubmitOptions) {
	// if opts == nil {
	// 	opts.Timedout = 60
	// }
	p.task <- f
}
