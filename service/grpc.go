// @Version : 1.0
// @Author  : steven.wong
// @Email   : 'wangxk1991@gamil.com'
// @Time    : 2024/01/19 15:28:53
// Desc     :

package service

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
)

func GRPCAPIService(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	logrus.Infof("GRPC API service stopped")
}
