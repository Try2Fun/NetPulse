// Package hub gestiona las conexiones WebSocket activas.
//
// Implementa el patrón Hub clásico de Go para WebSockets:
//   - Registra/desregistra clientes cuando el navegador abre/cierra la pestaña.
//   - Hace broadcast de mensajes JSON a TODOS los clientes conectados simultáneamente.
//   - Es seguro para uso concurrente (goroutine-safe).
package hub

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// Client representa una conexión WebSocket de un navegador individual.
type Client struct {
	Conn *websocket.Conn
	Send chan []byte
}

// Hub mantiene el registro de todos los clientes conectados y coordina el broadcast.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
	mu         sync.RWMutex
}

// New crea un Hub listo para usar. Llama a Run() en una goroutine separada.
func New() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 512),
		Register:   make(chan *Client, 32),
		Unregister: make(chan *Client, 32),
	}
}

// Run es el loop principal del Hub. Debe ejecutarse en su propia goroutine.
//
// Escucha tres canales concurrentemente:
//   - Register:   un nuevo navegador abrió la página
//   - Unregister: un navegador cerró la pestaña
//   - broadcast:  un mensaje listo para enviar a todos
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[HUB] Cliente conectado. Total: %d", h.clientCount())

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("[HUB] Cliente desconectado. Total: %d", h.clientCount())

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.Send <- msg:
				default:
					// El cliente es demasiado lento — descartamos para no bloquear el hub
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast encola un mensaje para enviar a todos los clientes conectados.
// Es no-bloqueante: si el canal de broadcast está lleno, se descarta el mensaje.
func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
		// Canal lleno — el monitor es más rápido que los clientes
	}
}

// clientCount retorna el número actual de clientes conectados (thread-safe).
func (h *Hub) clientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
