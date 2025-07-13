/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wwangxiaoakng@modelbest.cn'
 @Time    : 2024/04/30 10:27:44
 Desc     :
*/

package event

import (
	"context"
	"fmt"
	"sync"

	"github.com/panjf2000/ants/v2"
	"github.com/piaobeizu/titan/config"
	"github.com/sirupsen/logrus"
)

type MsgKind string

// type EventName string

type Message struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Action string `json:"action"`
	Data   any    `json:"data"`
}

func (m *Message) String() string {
	return fmt.Sprintf("[event] from: %s, to: %s, action: %s, data: %v", m.From, m.To, m.Action, m.Data)
}

func NewMessage(from, to, action string, data any) *Message {
	return &Message{
		From:   from,
		To:     to,
		Action: action,
		Data:   data,
	}
}

type Event struct {
	mu       sync.RWMutex
	ctx      context.Context
	config   *config.Event
	pool     *ants.Pool
	messages map[string]chan *Message
	funcs    map[string][]*Func
}

type Func struct {
	ctx    context.Context
	action string
	pool   *ants.Pool
	sync   bool
	panic  bool
	f      func(param any) error
	done   chan struct{}
}

func NewFunc(ctx context.Context, action string, f func(param any) error) *Func {
	subctx := context.WithoutCancel(ctx)
	return &Func{
		ctx:    subctx,
		action: action,
		f:      f,
	}
}

func (F *Func) Do(msg *Message) (err error) {
	if F.sync {
		for {
			select {
			case <-F.ctx.Done():
				logrus.WithFields(logrus.Fields{
					"func": F.action,
				}).Warnf("context done, msg: %s", msg)
				return nil
			default:
				err = F.f(msg.Data)
				if err != nil && F.panic {
					panic(err)
				}
				return err
			}
		}
	}
	F.pool.Submit(func() {
		for {
			select {
			case <-F.ctx.Done():
				logrus.WithFields(logrus.Fields{
					"func": F.action,
				}).Warnf("context done, msg: %s", msg)
				F.done <- struct{}{}
				return
			default:
				err = F.f(msg.Data)
				F.done <- struct{}{}
			}
		}
	})
	<-F.done
	if err != nil && F.panic {
		panic(err)
	}
	return err
}

func (F *Func) Sync(sync bool) *Func {
	if F == nil {
		panic("nil func")
	}
	F.sync = sync
	return F
}

func (F *Func) Panic(p bool) *Func {
	if F == nil {
		panic("nil func")
	}
	F.panic = p
	return F
}

var (
	event *Event
)

func NewEvent(ctx context.Context, conf *config.Event, pool *ants.Pool) *Event {
	if event != nil {
		return event
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if conf == nil {
		conf = &config.Event{
			MsgSize: 1000,
		}
	}
	if pool == nil {
		pool, _ = ants.NewPool(1000)
	}
	event = &Event{
		ctx:      ctx,
		mu:       sync.RWMutex{},
		config:   conf,
		pool:     pool,
		messages: make(map[string]chan *Message, 4),
		funcs:    make(map[string][]*Func, 4),
	}
	return event
}

func (q *Event) Register(name string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.messages[name]; ok {
		return
	}
	q.messages[name] = make(chan *Message, q.config.MsgSize)
}

func (q *Event) Funcs(name string, action string) *Func {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if _, ok := q.funcs[name]; !ok {
		panic(fmt.Sprintf("func %s not found", name))
	}
	for _, f := range q.funcs[name] {
		if f.action == action {
			return f
		}
	}
	panic(fmt.Sprintf("action %s.%s not found", name, action))
}

func (q *Event) AddFunc(name string, F *Func) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if _, ok := q.funcs[name]; !ok {
		q.funcs[name] = []*Func{}
	}
	q.funcs[name] = append(q.funcs[name], F)
}

// Remove removes a name from the queue
func (q *Event) Remove(name string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.messages[name]; !ok {
		return fmt.Errorf("queue %s not found", name)
	}
	close(q.messages[name])
	delete(q.messages, name)
	return nil
}

func (q *Event) Send(msg *Message) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if _, ok := q.messages[msg.To]; !ok {
		return fmt.Errorf("queue %s not found", msg.To)
	}
	if len(q.messages[msg.To]) >= q.config.MsgSize {
		logrus.WithFields(logrus.Fields{
			"from": msg.From,
			"to":   msg.To,
			"size": len(q.messages[msg.To]),
		}).Warnf("queue %s is full", msg.To)
		<-q.messages[msg.To]
	}
	q.messages[msg.To] <- msg
	return nil
}

func (q *Event) Receive(name string) chan *Message {
	return q.messages[name]
}

func (q *Event) GetEvents() map[string]chan *Message {
	return q.messages
}

func (q *Event) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	names := []string{}
	for k := range q.messages {
		names = append(names, k)
	}
	for _, k := range names {
		close(q.messages[k])
		delete(q.messages, k)
	}
}

func (q *Event) Status() map[string]int {
	ret := make(map[string]int, 1)
	for k, v := range q.messages {
		ret[k] = len(v)
	}
	return ret
}
