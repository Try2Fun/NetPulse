// Package notifier implementa el sistema de alertas de NetPulse.
//
// # Diseño general
//
// El notificador tiene dos responsabilidades:
//  1. Decidir CUÁNDO disparar una alerta (lógica de estado por target).
//  2. Enviar la alerta al canal configurado (Telegram o Webhook HTTP genérico).
//
// Para (1) usamos un mapa de "estado previo" por target. Esto evita spam:
// si el target ya estaba DOWN, no se re-notifica en cada ciclo.
// Solo notificamos en transiciones: UP→DOWN/TIMEOUT y DOWN/TIMEOUT→UP (recovery).
//
// Para (2) implementamos la interfaz Sender, que permite agregar canales nuevos
// (Slack, PagerDuty, email) sin modificar el Notifier central.
//
//	┌─────────────────────────────────────────────────────────────┐
//	│  Engine.Run() ─► []PingResult ─► Notifier.Process()        │
//	│                                         │                   │
//	│                              transición │ detectada?        │
//	│                                         ▼                   │
//	│                              Sender.Send(Alert)             │
//	│                              ├── TelegramSender.Send()      │
//	│                              └── WebhookSender.Send()       │
//	└─────────────────────────────────────────────────────────────┘
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/tuusuario/netpulse/internal/pinger"
)

// =============================================================================
// INTERFACES Y TIPOS PÚBLICOS
// =============================================================================

// Alert contiene toda la información relevante de un evento de alerta.
// Se pasa a los Sender para que puedan formatear el mensaje como quieran.
type Alert struct {
	// TargetName es el nombre legible del host afectado.
	TargetName string

	// Address es la IP o hostname del host.
	Address string

	// OldStatus es el estado anterior (antes del cambio).
	OldStatus pinger.Status

	// NewStatus es el estado actual que disparó la alerta.
	NewStatus pinger.Status

	// Latency es el último tiempo de respuesta medido (0 si está caído).
	Latency time.Duration

	// Timestamp es el momento exacto en que se detectó el cambio.
	Timestamp time.Time

	// IsRecovery es true cuando el host vuelve a UP desde un estado de falla.
	// Permite que los Sender usen emojis/mensajes diferentes para recuperaciones.
	IsRecovery bool
}

// Sender es la interfaz que deben implementar todos los canales de notificación.
//
// Por qué una interfaz y no un switch/case:
// El principio Open/Closed dice que el código debe estar abierto a extensión
// pero cerrado a modificación. Con Sender, agregar Slack no toca el Notifier.
type Sender interface {
	// Send envía la alerta al canal externo.
	// Retorna error si el envío falló (log, no panic).
	Send(alert Alert) error

	// Name retorna el nombre del canal (para logging).
	Name() string
}

// =============================================================================
// NOTIFIER — ORQUESTADOR CENTRAL
// =============================================================================

// Notifier detecta transiciones de estado y despacha alertas a los Senders.
type Notifier struct {
	// senders es la lista de canales de notificación activos.
	senders []Sender

	// prevStatus guarda el último estado conocido por target (clave = Address).
	// Usamos Address como clave porque es más estable que el Name.
	//
	// Nota sobre concurrencia: Process() se llama desde la goroutine principal
	// de main.go (el loop del Ticker), por lo que no necesitamos mutex aquí.
	// Si en el futuro se llama desde múltiples goroutines, hay que agregar sync.RWMutex.
	prevStatus map[string]pinger.Status
}

// New crea un Notifier con los Senders indicados.
//
// Uso típico:
//
//	n := notifier.New(
//	    notifier.NewTelegramSender(token, chatID),
//	    notifier.NewWebhookSender(webhookURL),
//	)
func New(senders ...Sender) *Notifier {
	return &Notifier{
		senders:    senders,
		prevStatus: make(map[string]pinger.Status),
	}
}

// Process recibe los resultados de un ciclo de ping y dispara alertas
// únicamente cuando detecta un CAMBIO de estado en algún target.
//
// # Lógica de transición de estados
//
//	Estado previo │ Estado actual │ ¿Alerta?
//	──────────────┼───────────────┼──────────────────────────────────
//	(sin estado)  │ UP            │ No (inicio limpio, sin transición)
//	(sin estado)  │ DOWN/TIMEOUT  │ Sí (caída detectada desde el inicio)
//	UP            │ UP            │ No (sin cambio)
//	UP            │ DOWN/TIMEOUT  │ Sí (caída nueva)
//	DOWN/TIMEOUT  │ UP            │ Sí (recuperación)
//	DOWN/TIMEOUT  │ DOWN/TIMEOUT  │ No (ya se notificó antes)
func (n *Notifier) Process(results []pinger.PingResult) {
	for _, r := range results {
		prev, seen := n.prevStatus[r.Address]

		// ── Caso 1: primera vez que vemos este target ──────────────────────────
		if !seen {
			n.prevStatus[r.Address] = r.Status
			// Solo alertamos si arranca en estado de falla.
			// Si arranca UP, consideramos que es el estado "normal" inicial.
			if r.Status != pinger.StatusUp {
				n.dispatch(Alert{
					TargetName: r.TargetName,
					Address:    r.Address,
					OldStatus:  pinger.Status("UNKNOWN"),
					NewStatus:  r.Status,
					Latency:    r.Latency,
					Timestamp:  r.Timestamp,
					IsRecovery: false,
				})
			}
			continue
		}

		// ── Caso 2: mismo estado que antes — sin cambio, sin alerta ───────────
		if prev == r.Status {
			continue
		}

		// ── Caso 3: hubo cambio de estado ─────────────────────────────────────
		isRecovery := r.Status == pinger.StatusUp
		n.dispatch(Alert{
			TargetName: r.TargetName,
			Address:    r.Address,
			OldStatus:  prev,
			NewStatus:  r.Status,
			Latency:    r.Latency,
			Timestamp:  r.Timestamp,
			IsRecovery: isRecovery,
		})

		// Actualizamos el estado previo para el próximo ciclo.
		n.prevStatus[r.Address] = r.Status
	}
}

// dispatch envía la alerta a todos los Senders registrados.
// Itera todos aunque alguno falle — un fallo en Telegram no debe bloquear el webhook.
func (n *Notifier) dispatch(alert Alert) {
	for _, s := range n.senders {
		if err := s.Send(alert); err != nil {
			// Logueamos el error pero no interrumpimos el flujo.
			log.Printf("[NOTIFIER] Error enviando alerta por %s: %v", s.Name(), err)
		}
	}
}

// =============================================================================
// TELEGRAM SENDER
// =============================================================================

// TelegramSender envía alertas al Bot API de Telegram usando sendMessage.
//
// # Cómo obtener token y chatID:
//  1. Habla con @BotFather en Telegram → /newbot → copia el token.
//  2. Envía un mensaje a tu bot, luego visita:
//     https://api.telegram.org/bot<TOKEN>/getUpdates
//     Busca "chat":{"id": ...} — ese es tu chatID.
type TelegramSender struct {
	token  string // Token del bot: "123456:ABC-DEF..."
	chatID string // ID del chat/canal destino: "-1001234567890"
	client *http.Client
}

// NewTelegramSender crea un TelegramSender listo para usar.
func NewTelegramSender(token, chatID string) *TelegramSender {
	return &TelegramSender{
		token:  token,
		chatID: chatID,
		// Cliente HTTP con timeout razonable.
		// Sin timeout, una red lenta puede bloquear el ciclo de monitoreo indefinidamente.
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Name implementa Sender.Name().
func (t *TelegramSender) Name() string { return "Telegram" }

// Send implementa Sender.Send() para el Bot API de Telegram.
//
// Endpoint: POST https://api.telegram.org/bot<TOKEN>/sendMessage
// Body: JSON con chat_id, text y parse_mode=MarkdownV2.
func (t *TelegramSender) Send(alert Alert) error {
	text := formatTelegramMessage(alert)

	// Struct anónima: evitamos definir un tipo que solo se usa aquí.
	payload := struct {
		ChatID    string `json:"chat_id"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode"`
	}{
		ChatID:    t.chatID,
		Text:      text,
		ParseMode: "MarkdownV2",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram: error codificando payload: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.token)

	resp, err := t.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: error en POST: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: respuesta inesperada: HTTP %d", resp.StatusCode)
	}

	log.Printf("[TELEGRAM] Alerta enviada para %s (%s→%s)", alert.TargetName, alert.OldStatus, alert.NewStatus)
	return nil
}

// formatTelegramMessage genera el texto de la alerta en formato MarkdownV2.
//
// MarkdownV2 requiere escapar caracteres especiales como . - ( ) ! = + # _ *
// Usamos backticks para código inline (no necesitan escape interno).
func formatTelegramMessage(alert Alert) string {
	emoji := "🔴"
	statusLabel := "CAÍDA DETECTADA"
	if alert.IsRecovery {
		emoji = "🟢"
		statusLabel = "RECUPERACIÓN"
	} else if alert.NewStatus == pinger.StatusTimeout {
		emoji = "🟡"
		statusLabel = "TIMEOUT"
	}

	ts := alert.Timestamp.Format("02/01/2006 15:04:05")

	msg := fmt.Sprintf(
		"%s *NetPulse \\- %s*\n\n"+
			"🎯 Host: `%s`\n"+
			"📡 Dirección: `%s`\n"+
			"🔄 Estado: `%s` → `%s`\n"+
			"⏱ Hora: `%s`",
		emoji, statusLabel,
		alert.TargetName,
		alert.Address,
		alert.OldStatus,
		alert.NewStatus,
		ts,
	)

	if alert.IsRecovery && alert.Latency > 0 {
		msg += fmt.Sprintf("\n📶 Latencia: `%s`", alert.Latency.Round(time.Millisecond))
	}

	return msg
}

// =============================================================================
// WEBHOOK HTTP SENDER
// =============================================================================

// WebhookSender envía alertas a cualquier URL HTTP como un POST con body JSON.
//
// Compatible con n8n, Zapier, Make, Slack Incoming Webhooks, Discord Webhooks,
// y cualquier endpoint REST que acepte JSON.
type WebhookSender struct {
	url    string
	client *http.Client
}

// NewWebhookSender crea un WebhookSender que enviará a la URL indicada.
func NewWebhookSender(url string) *WebhookSender {
	return &WebhookSender{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Name implementa Sender.Name().
func (w *WebhookSender) Name() string { return "Webhook" }

// webhookPayload es el cuerpo JSON que enviamos al webhook.
// Usamos snake_case en json tags para ser compatibles con la mayoría de APIs REST.
type webhookPayload struct {
	Event      string `json:"event"`       // "alert" o "recovery"
	TargetName string `json:"target_name"` // Nombre descriptivo del host
	Address    string `json:"address"`     // IP o hostname
	OldStatus  string `json:"old_status"`  // Estado anterior
	NewStatus  string `json:"new_status"`  // Estado actual
	LatencyMs  int64  `json:"latency_ms"`  // Latencia en ms (0 si caído)
	Timestamp  string `json:"timestamp"`   // ISO 8601 UTC
}

// Send implementa Sender.Send() para webhooks HTTP genéricos.
func (w *WebhookSender) Send(alert Alert) error {
	event := "alert"
	if alert.IsRecovery {
		event = "recovery"
	}

	payload := webhookPayload{
		Event:      event,
		TargetName: alert.TargetName,
		Address:    alert.Address,
		OldStatus:  string(alert.OldStatus),
		NewStatus:  string(alert.NewStatus),
		LatencyMs:  alert.Latency.Milliseconds(),
		Timestamp:  alert.Timestamp.UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook: error codificando payload: %w", err)
	}

	resp, err := w.client.Post(w.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: error en POST a '%s': %w", w.url, err)
	}
	defer resp.Body.Close()

	// Consideramos éxito cualquier 2xx.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook: respuesta no-2xx desde '%s': HTTP %d", w.url, resp.StatusCode)
	}

	log.Printf("[WEBHOOK] Alerta enviada para %s (%s→%s)", alert.TargetName, alert.OldStatus, alert.NewStatus)
	return nil
}
