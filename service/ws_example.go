/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2025/05/30 16:00:00
 Desc     : WebSocket 事件处理器使用示例
*/

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// WebSocket 事件处理器使用示例
func ExampleWebSocketEventHandlers() {
	ctx := context.Background()
	log := logrus.WithFields(logrus.Fields{"component": "websocket"})

	// 创建 WebSocket 服务器
	server := NewWebSocketServer(ctx, "localhost:8080", "v1", log)

	// 注册客户端连接事件处理器
	server.OnEvent(ServerEventClientConnected, func(event ServerEvent) {
		log.Infof("客户端 %s 已连接，当前总连接数: %v",
			event.ClientID, event.Metadata["total_connections"])

		// 可以在这里执行自定义逻辑，比如：
		// - 发送欢迎消息
		// - 记录用户上线日志
		// - 更新在线用户统计
		welcomeMsg := WSMessage[string]{
			Type:      WSMSG_TYPE_WELCOME,
			Data:      fmt.Sprintf("欢迎客户端 %s！", event.ClientID),
			Timestamp: time.Now().Unix(),
			ClientID:  event.ClientID,
		}
		server.SendJSON(event.ClientID, welcomeMsg)
	})

	// 注册客户端断开事件处理器
	server.OnEvent(ServerEventClientDisconnected, func(event ServerEvent) {
		log.Warnf("客户端 %s 已断开连接，当前总连接数: %v",
			event.ClientID, event.Metadata["total_connections"])

		// 可以在这里执行清理逻辑，比如：
		// - 记录用户下线日志
		// - 清理用户相关数据
		// - 通知其他客户端用户离线
	})

	// 注册消息接收事件处理器
	server.OnEvent(ServerEventMessageReceived, func(event ServerEvent) {
		log.Infof("收到来自客户端 %s 的消息: %v", event.ClientID, event.Data)

		// 可以在这里处理业务逻辑，比如：
		// - 消息路由
		// - 数据处理
		// - 消息转发

		// 示例：回显消息
		response := WSMessage[any]{
			Type:      WSMSG_TYPE_SUCCESS,
			Data:      fmt.Sprintf("服务器已收到您的消息: %v", event.Data),
			Timestamp: time.Now().Unix(),
			ClientID:  event.ClientID,
		}
		server.SendJSON(event.ClientID, response)
	})

	// 注册消息发送事件处理器
	server.OnEvent(ServerEventMessageSent, func(event ServerEvent) {
		log.Debugf("向客户端 %s 发送消息成功: %v", event.ClientID, event.Data)

		// 可以在这里记录发送统计或执行其他逻辑
	})

	// 注册广播事件处理器
	server.OnEvent(ServerEventBroadcastSent, func(event ServerEvent) {
		successCount := event.Metadata["success_count"]
		failCount := event.Metadata["fail_count"]
		totalClients := event.Metadata["total_clients"]

		log.Infof("广播消息完成 - 成功: %v, 失败: %v, 总计: %v",
			successCount, failCount, totalClients)

		// 可以在这里记录广播统计
	})

	// 注册错误事件处理器
	server.OnEvent(ServerEventError, func(event ServerEvent) {
		if event.ClientID != "" {
			log.Errorf("客户端 %s 发生错误: %v", event.ClientID, event.Error)
		} else {
			log.Errorf("服务器发生错误: %v", event.Error)
		}

		// 可以在这里执行错误处理逻辑，比如：
		// - 错误报告
		// - 自动重试
		// - 告警通知
	})

	// 启动广播处理
	go server.handleBroadcast()

	log.Info("WebSocket 服务器已启动，事件处理器已注册")
}

// 高级事件处理器示例 - 连接统计和监控
func SetupConnectionMonitoring(server *WebSocketServer) {
	connectionStats := struct {
		totalConnections    int64
		totalDisconnections int64
		currentConnections  int64
		peakConnections     int64
	}{}

	// 连接监控
	server.OnEvent(ServerEventClientConnected, func(event ServerEvent) {
		connectionStats.totalConnections++
		connectionStats.currentConnections++

		if connectionStats.currentConnections > connectionStats.peakConnections {
			connectionStats.peakConnections = connectionStats.currentConnections
		}

		logrus.WithFields(logrus.Fields{
			"client_id":           event.ClientID,
			"current_connections": connectionStats.currentConnections,
			"total_connections":   connectionStats.totalConnections,
			"peak_connections":    connectionStats.peakConnections,
		}).Info("连接统计更新")
	})

	server.OnEvent(ServerEventClientDisconnected, func(event ServerEvent) {
		connectionStats.totalDisconnections++
		connectionStats.currentConnections--

		logrus.WithFields(logrus.Fields{
			"client_id":            event.ClientID,
			"current_connections":  connectionStats.currentConnections,
			"total_disconnections": connectionStats.totalDisconnections,
		}).Info("断开连接统计更新")
	})
}

// 消息频率限制示例
func SetupRateLimiting(server *WebSocketServer) {
	clientMessageCount := make(map[string]int)
	resetTimer := time.NewTicker(time.Minute) // 每分钟重置一次计数

	go func() {
		for range resetTimer.C {
			clientMessageCount = make(map[string]int)
		}
	}()

	server.OnEvent(ServerEventMessageReceived, func(event ServerEvent) {
		clientMessageCount[event.ClientID]++

		// 限制每分钟最多100条消息
		if clientMessageCount[event.ClientID] > 100 {
			logrus.Warnf("客户端 %s 消息频率过高，当前计数: %d",
				event.ClientID, clientMessageCount[event.ClientID])

			// 发送警告消息
			warningMsg := WSMessage[string]{
				Type:      WSMSG_TYPE_ERROR,
				Data:      "消息频率过高，请稍后再试",
				Timestamp: time.Now().Unix(),
				ClientID:  event.ClientID,
			}
			server.SendJSON(event.ClientID, warningMsg)
		}
	})
}

// 自动重连提醒示例
func SetupReconnectionReminder(server *WebSocketServer) {
	server.OnEvent(ServerEventClientDisconnected, func(event ServerEvent) {
		// 模拟向客户端发送重连提醒（实际应用中可能通过其他方式）
		logrus.Infof("客户端 %s 断开连接，建议检查网络并重新连接", event.ClientID)

		// 可以在这里触发：
		// - 推送通知
		// - 邮件提醒
		// - 短信通知等
	})
}
