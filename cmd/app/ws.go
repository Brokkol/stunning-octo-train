package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// Доступ и так защищён токеном, поэтому Origin не проверяем
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Hub хранит все открытые соединения и рассылает им сообщения
type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]string // соединение -> uid владельца
}

func NewHub() *Hub {
	return &Hub{clients: map[*websocket.Conn]string{}}
}

func (h *Hub) add(c *websocket.Conn, uid string) {
	h.mu.Lock()
	h.clients[c] = uid
	h.mu.Unlock()
}

func (h *Hub) remove(c *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	c.Close()
}

// Broadcast отправляет сообщение всем подключённым
func (h *Hub) Broadcast(uid string, msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c, owner := range h.clients {
		if owner != uid {
			continue
		}
		c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
			c.Close()
			delete(h.clients, c)
		}
	}
}

// PingLoop раз в 30 секунд «стучится» в каждое соединение,
// чтобы nginx и Cloudflare не считали его заброшенным
func (h *Hub) PingLoop() {
	for range time.Tick(30 * time.Second) {
		h.mu.Lock()
		for c := range h.clients {
			c.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.Close()
				delete(h.clients, c)
			}
		}
		h.mu.Unlock()
	}
}

// ServeWS превращает обычный запрос в WebSocket и держит его открытым
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.add(conn, user.UID)
	defer h.remove(conn)

	// Читаем, только чтобы заметить, что клиент отключился
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
