/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2024/01/21 11:07:45
 Desc     :
*/

package service

import (
	"context"
	"net/http"
	"sync"
)

var STATIC_DIR = "./static/"

type HttpServer struct {
	ctx     context.Context
	server  *http.Server
	WebAddr string
}

func HTTPWebService(ctx context.Context, wg *sync.WaitGroup) {
	// logrus.Infof("start web service, listen on %s", config.GetConfig().Http.WebAddr)
	// defer wg.Done()
	// //文件浏览
	// http.Header{}.Add("Access-Control-Allow-Origin", "*")
	// http.Handle("/", http.FileServer(http.Dir(STATIC_DIR)))
	// err := http.ListenAndServe(cfg.Http.WebAddr, nil)
	// if err != nil {
	// 	log.Fatal("ListenAndServe: ", err)
	// }
	// // Graceful shutdown
	// <-ctx.Done()
	// logrus.Infof("HTTP WEB service stopped")
}
