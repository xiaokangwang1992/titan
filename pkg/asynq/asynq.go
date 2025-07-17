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

type AsynqTask struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

func NewQTask(server *asynq.Server) *AsynqTask {
	return &AsynqTask{
		server: server,
		mux:    asynq.NewServeMux(),
	}
}

func (q *AsynqTask) RegisterHandler(name string, handler asynq.HandlerFunc) {
	q.mux.HandleFunc(name, handler)
}

func (q *AsynqTask) Start() {
	if err := q.server.Run(q.mux); err != nil {
		logrus.Fatal(err)
	}
}
