/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2024/10/30 18:53:08
 Desc     :
*/

package service

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type ClientChan chan string

// It keeps a list of clients those are currently attached
// and broadcasting events to those clients.
type event struct {
	// Events are pushed to this channel by the main events-gathering routine
	message chan string
	// New client connections
	newClients chan chan string
	// Closed client connections
	closedClients chan chan string
	// Total client connections
	totalClients map[chan string]bool
}

// Initialize event and Start procnteessing requests
func newEvent() (e *event) {
	e = &event{
		message:       make(chan string),
		newClients:    make(chan chan string),
		closedClients: make(chan chan string),
		totalClients:  make(map[chan string]bool),
	}
	go e.listen()
	return
}

// It Listens all incoming requests from clients.
// Handles addition and removal of clients and broadcast messages to clients.
func (stream *event) listen() {
	for {
		select {
		// Add new available client
		case client := <-stream.newClients:
			stream.totalClients[client] = true
			logrus.Infof("Client added. %d registered clients", len(stream.totalClients))
		// Remove closed client
		case client := <-stream.closedClients:
			delete(stream.totalClients, client)
			close(client)
			logrus.Infof("Removed client. %d registered clients", len(stream.totalClients))
		// Broadcast message to client
		case eventMsg := <-stream.message:
			for clientMessageChan := range stream.totalClients {
				clientMessageChan <- eventMsg
			}
		}
	}
}

func (stream *event) serveHTTP() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Initialize client channel
		clientChan := make(ClientChan)
		// Send new connection to event server
		stream.newClients <- clientChan
		defer func() {
			// Send closed connection to event server
			stream.closedClients <- clientChan
		}()
		c.Set("sse-client", clientChan)
		c.Next()
	}
}

func sseHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked")
		c.Next()
	}
}
