// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the MIT License. See LICENSE file for details.
package ws

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second
	maxMsgSize = 4096
)

type Conn struct {
	ws     *websocket.Conn
	send   chan []byte
	UserID int64
}

func NewConn(ws *websocket.Conn, userID int64) *Conn {
	return &Conn{
		ws:     ws,
		send:   make(chan []byte, 64),
		UserID: userID,
	}
}

func (c *Conn) Send(data []byte) {
	select {
	case c.send <- data:
	default:
		log.Printf("[ws] dropping message for user %d (buffer full)", c.UserID)
	}
}

func (c *Conn) ReadPump(hub *Hub, onMessage func(userID int64, data []byte)) {
	defer func() {
		hub.Unregister(c.UserID, c)
		c.ws.Close()
	}()
	c.ws.SetReadLimit(maxMsgSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		if onMessage != nil {
			onMessage(c.UserID, msg)
		}
	}
}

func (c *Conn) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
