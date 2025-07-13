/*
 * @Version : 1.0
 * @Author  : xiaokang.w
 * @Email   : xiaokang.w@gmicloud.ai
 * @Date    : 2025/07/13
 * @Desc    : 描述信息
 */

package qtask

import (
	"github.com/hibiken/asynq"
	"github.com/sirupsen/logrus"
)

type QTask struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

func NewQTask(server *asynq.Server) *QTask {
	return &QTask{
		server: server,
		mux:    asynq.NewServeMux(),
	}
}

func (q *QTask) RegisterHandler(name string, handler asynq.HandlerFunc) {
	q.mux.HandleFunc(name, handler)
}

func (q *QTask) Start() {
	if err := q.server.Run(q.mux); err != nil {
		logrus.Fatal(err)
	}
}
