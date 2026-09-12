// Package server implementa el servidor HTTP + WebSocket del dashboard de NetPulse.
//
// Responsabilidades:
//  1. Servir los archivos web estáticos (HTML/CSS/JS) embebidos en el binario.
//  2. Manejar conexiones WebSocket desde el navegador.
//  3. Recibir resultados de cada ciclo de monitoreo y hacer broadcast en formato JSON.
//  4. Detectar nuevos dispositivos en tiempo real y emitir eventos "new_device".
//  5. Persistir nuevos dispositivos automáticamente en targets.json.
package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tuusuario/netpulse/config"
	"github.com/tuusuario/netpulse/internal/hub"
	"github.com/tuusuario/netpulse/internal/pinger"
)

// ─────────────────────────────────────────────
// Tipos de mensajes JSON (protocolo WebSocket)
// ─────────────────────────────────────────────

// CycleMessage se envía tras cada ciclo de monitoreo completo.
type CycleMessage struct {
	Type      string       `json:"type"`      // siempre "cycle_update"
	Timestamp string       `json:"timestamp"` // ISO 8601 UTC
	CycleMs   int64        `json:"cycle_ms"`  // duración del ciclo en ms
	Stats     Stats        `json:"stats"`
	Devices   []DeviceJSON `json:"devices"`
}

// NewDeviceMessage se envía cuando se detecta un dispositivo nuevo por primera vez.
type NewDeviceMessage struct {
	Type   string     `json:"type"` // siempre "new_device"
	Device DeviceJSON `json:"device"`
}

// Stats resume el estado global de la red en el ciclo.
type Stats struct {
	Total int `json:"total"`
	Up    int `json:"up"`
	Down  int `json:"down"`
}

// DeviceJSON es la representación JSON de un dispositivo para el dashboard.
type DeviceJSON struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	IsNew     bool   `json:"is_new"`
	Icon      string `json:"icon"`
}

// ─────────────────────────────────────────────
// Server
// ─────────────────────────────────────────────

// Server orquesta el servidor web y la lógica de tiempo real del dashboard.
type Server struct {
	h            *hub.Hub
	cfg          *config.Config
	configPath   string
	knownDevices map[string]bool // IPs ya vistas en algún ciclo
	knownMu      sync.Mutex
	lastCycle    []byte
	lastCycleMu  sync.RWMutex
}

var upgrader = websocket.Upgrader{
	// Permitimos cualquier origen para acceder desde la LAN local
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 8192,
}

// New crea un Server listo para usar.
func New(h *hub.Hub, cfg *config.Config, configPath string) *Server {
	// Sembramos knownDevices con los targets ya existentes en el JSON
	known := make(map[string]bool)
	for _, t := range cfg.Targets {
		clean := strings.Split(t.Address, ":")[0]
		known[clean] = true
	}

	return &Server{
		h:            h,
		cfg:          cfg,
		configPath:   configPath,
		knownDevices: known,
	}
}

// Start arranca el servidor HTTP en el puerto indicado.
// Debe llamarse en su propia goroutine: go srv.Start(8080).
func (s *Server) Start(port int, webFS fs.FS) error {
	mux := http.NewServeMux()

	// Archivos estáticos (embedded)
	mux.Handle("/", http.FileServer(http.FS(webFS)))
	mux.HandleFunc("/ws", s.handleWS)

	go s.h.Run()

	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, mux)
}

// handleWS maneja el upgrade HTTP→WebSocket y gestiona el ciclo de vida del cliente.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Error en upgrade: %v", err)
		return
	}

	client := &hub.Client{
		Conn: conn,
		Send: make(chan []byte, 256),
	}
	s.h.Register <- client

	s.lastCycleMu.RLock()
	if s.lastCycle != nil {
		client.Send <- s.lastCycle
	}
	s.lastCycleMu.RUnlock()

	// Goroutine escritora: envía mensajes del canal Send al navegador
	go func() {
		defer func() {
			s.h.Unregister <- client
			conn.Close()
		}()
		for msg := range client.Send {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// Goroutine lectora: mantiene la conexión viva y detecta desconexiones
	go func() {
		defer func() {
			s.h.Unregister <- client
			conn.Close()
		}()
		conn.SetReadLimit(512)
		// Se quitó el ReadDeadline para evitar que se desconecte si el cliente web no envía Pings
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

// BroadcastCycle serializa los resultados de un ciclo y los envía a todos los navegadores.
// También detecta dispositivos nuevos y emite eventos "new_device" + persiste al JSON.
func (s *Server) BroadcastCycle(results []pinger.PingResult, elapsed time.Duration) {
	up, down := 0, 0
	var devs []DeviceJSON
	var newEvents [][]byte

	s.knownMu.Lock()
	for _, r := range results {
		cleanAddr := strings.Split(r.Address, ":")[0]
		isNew := !s.knownDevices[cleanAddr]
		if isNew {
			s.knownDevices[cleanAddr] = true
		}

		if r.Status == pinger.StatusUp {
			up++
		} else {
			down++
		}

		d := DeviceJSON{
			Name:      r.TargetName,
			Address:   r.Address,
			Status:    strings.ToLower(string(r.Status)),
			LatencyMs: r.Latency.Milliseconds(),
			IsNew:     isNew,
			Icon:      inferIcon(r.TargetName, r.Address),
		}
		devs = append(devs, d)

		if isNew {
			nd, _ := json.Marshal(NewDeviceMessage{Type: "new_device", Device: d})
			newEvents = append(newEvents, nd)
			go s.persistDevice(r)
		}
	}
	s.knownMu.Unlock()

	// Broadcast del ciclo completo
	cycleMsg := CycleMessage{
		Type:      "cycle_update",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		CycleMs:   elapsed.Milliseconds(),
		Stats:     Stats{Total: len(devs), Up: up, Down: down},
		Devices:   devs,
	}
	data, _ := json.Marshal(cycleMsg)

	s.lastCycleMu.Lock()
	s.lastCycle = data
	s.lastCycleMu.Unlock()

	s.h.Broadcast(data)

	// Broadcast de nuevos dispositivos (después del ciclo para que la tarjeta ya exista)
	for _, nd := range newEvents {
		s.h.Broadcast(nd)
	}
}

// persistDevice agrega un dispositivo recién descubierto al targets.json de forma segura.
func (s *Server) persistDevice(r pinger.PingResult) {
	s.cfg.TargetsMu.Lock()
	defer s.cfg.TargetsMu.Unlock()

	clean := strings.Split(r.Address, ":")[0]
	for _, t := range s.cfg.Targets {
		if strings.Split(t.Address, ":")[0] == clean {
			return // ya está en la lista
		}
	}

	s.cfg.Targets = append(s.cfg.Targets, config.Target{
		Name:    r.TargetName,
		Address: r.Address,
	})

	if err := s.cfg.Save(s.configPath); err != nil {
		log.Printf("[SERVER] ⚠️ No se pudo guardar dispositivo %s: %v", r.TargetName, err)
	} else {
		log.Printf("[SERVER] 💾 Nuevo dispositivo guardado: %s (%s)", r.TargetName, r.Address)
	}
}

// inferIcon deduce el ícono del dispositivo a partir de su nombre y dirección.
func inferIcon(name, addr string) string {
	n := strings.ToLower(name)
	switch {
	case contains(n, "playstation", "ps5", "ps4", "xbox", "gamepad", "nintendo"):
		return "gamepad"
	case contains(n, "tv", "roku", "tcl", "firetv", "chromecast", "bravia", "smarttv"):
		return "tv"
	case contains(n, "phone", "mobile", "poco", "xiaomi", "iphone", "android", "samsung", "galaxy", "redmi", "huawei"):
		return "phone"
	case contains(n, "router", "gateway", "modem", "zte", "tp-link", "mikrotik", "unifi"):
		return "router"
	case contains(n, "pc", "laptop", "desktop", "computer", "mac", "imac", "macbook"):
		return "laptop"
	case contains(n, "raspberry", " pi"):
		return "raspberry"
	case contains(n, "printer"):
		return "printer"
	case contains(n, "camera", "cam", "nvr", "ipcam"):
		return "camera"
	case contains(n, "dns", "cloudflare", "google", "opendns"):
		return "cloud"
	default:
		// Si la IP es externa (no RFC1918) es un servidor WAN
		if !isPrivateIP(addr) {
			return "cloud"
		}
		return "device"
	}
}

func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func isPrivateIP(addr string) bool {
	clean := strings.Split(addr, ":")[0]
	return strings.HasPrefix(clean, "192.168.") ||
		strings.HasPrefix(clean, "10.") ||
		strings.HasPrefix(clean, "172.")
}
