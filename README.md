# ⚡ NetPulse

<div align="center">

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-22c55e?style=for-the-badge)
![Status](https://img.shields.io/badge/Status-Active-6366f1?style=for-the-badge)
![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20Windows%20%7C%20macOS-f59e0b?style=for-the-badge)

**Motor de telemetría de red en tiempo real — concurrente, reactivo y listo para producción.**

*Monitorea hosts, detecta caídas y despacha alertas sin saturar tu CPU.*

</div>

---

## 🖥️ Demo en vivo

```
╔══════════════════════════════════════════════════════════════════════════╗
║   NetPulse  │  12/09/2026 18:30:00  │  ciclo en 1.87s                   ║
╠══════════════════════════════════════════════════════════════════════════╣
║  TARGET                ║  DIRECCIÓN       ║  ESTADO      ║  LATENCIA    ║
╠══════════════════════════════════════════════════════════════════════════╣
║  Google DNS (HTTPS)    ║  8.8.8.8:443     ║  🟢 UP       ║  14ms        ║
║  Cloudflare DNS (HTTPS)║  1.1.1.1:443     ║  🟢 UP       ║  12ms        ║
║  OpenDNS (HTTPS)       ║  208.67.222.222  ║  🟢 UP       ║  31ms        ║
║  Router Local          ║  192.168.1.1:80  ║  🔴 DOWN     ║  —           ║
╚══════════════════════════════════════════════════════════════════════════╝
```

---

## 🏗️ Arquitectura

NetPulse sigue una arquitectura de **pipeline concurrente** inspirada en los principios de diseño de Go: *"No comuniques compartiendo memoria; comparte memoria comunicándote."*

```
┌─────────────────────────────────────────────────────────────────────┐
│                         NetPulse Engine                             │
│                                                                     │
│  main()                                                             │
│   ├─ config.Load()         → carga targets.json                     │
│   ├─ pinger.New(cfg)       → construye el motor concurrente         │
│   ├─ notifier.New(senders) → construye el despachador de alertas    │
│   └─ Loop [time.Ticker]                                             │
│        │                                                            │
│        ▼                                                            │
│  Engine.Run()                                                       │
│   ├─ goroutine #1 ──► pingTarget(host1) ──► channel ──┐            │
│   ├─ goroutine #2 ──► pingTarget(host2) ──► channel ──┤            │
│   └─ goroutine #N ──► pingTarget(hostN) ──► channel ──┘            │
│                                                  │                  │
│                                    sync.WaitGroup + close(ch)       │
│                                                  │                  │
│                                    []PingResult  ▼                  │
│  Notifier.Process(results)                                          │
│   ├─ Detecta transiciones UP↔DOWN (sin spam de alertas)            │
│   ├─ TelegramSender.Send(alert)  → Bot API de Telegram             │
│   └─ WebhookSender.Send(alert)   → n8n / Zapier / REST endpoint    │
│                                                                     │
│  renderTable(results)            → stdout con tabla Unicode         │
└─────────────────────────────────────────────────────────────────────┘
```

### Señales del OS — Graceful Shutdown

```
Ctrl+C / SIGTERM
      │
      ▼
signal.NotifyContext(ctx)
      │
      ▼
select {
  case <-ticker.C:   → ejecuta ciclo de monitoreo
  case <-ctx.Done(): → salida limpia, libera recursos
}
```

---

## ✨ Features

| Feature | Detalle |
|---|---|
| ⚡ **Concurrencia idiomática** | Goroutines + channels bufferizados. N targets = N goroutines paralelas. |
| 🛡️ **Sin tormentas de alertas** | Máquina de estados por target. Solo notifica en transiciones `UP→DOWN` y `DOWN→UP`. |
| 🔌 **Extensible por diseño** | Interfaz `Sender` — agrega Slack, PagerDuty o email sin tocar el core. |
| 📱 **Telegram nativo** | Bot API con MarkdownV2, emojis y detalle de latencia en recuperaciones. |
| 🔗 **Webhooks genéricos** | Compatible con n8n, Zapier, Make, Discord, Slack Incoming Webhooks. |
| ⏱️ **Tick preciso** | `time.NewTicker` garantiza intervalos fijos independientemente de cuánto tarde el ciclo. |
| 🧹 **Graceful shutdown** | `signal.NotifyContext` captura `SIGINT`/`SIGTERM` — nunca un `kill -9`. |
| 🔄 **Reintentos inteligentes** | `MaxRetries` con backoff de 500ms entre intentos antes de marcar DOWN. |
| 🔍 **Timeout vs DOWN** | Diferencia entre `TIMEOUT` (red lenta) y `DOWN` (host inalcanzable). |

---

## 🚀 Inicio Rápido

### Prerequisitos

- **Go 1.22+** — [descargar](https://go.dev/dl/)

### Instalación

```bash
git clone https://github.com/Try2Fun/netpulse.git
cd netpulse
go build -o netpulse ./cmd/netpulse
```

### Uso básico (solo consola)

```bash
./netpulse -config targets.json
```

### Con notificaciones a Telegram

```bash
./netpulse \
  -config targets.json \
  -telegram-token "123456:ABC-DEFxyz" \
  -telegram-chat "-1001234567890"
```

### Con webhook HTTP (n8n, Zapier, Make…)

```bash
./netpulse \
  -config targets.json \
  -webhook-url "https://n8n.tuservidor.com/webhook/abc123"
```

### Todos los flags

| Flag | Descripción | Default |
|---|---|---|
| `-config` | Ruta al archivo JSON de targets | `targets.json` |
| `-telegram-token` | Token del bot de Telegram | *(vacío)* |
| `-telegram-chat` | Chat ID de Telegram destino | *(vacío)* |
| `-webhook-url` | URL de webhook HTTP para alertas | *(vacío)* |

---

## ⚙️ Configuración (`targets.json`)

```json
{
  "targets": [
    { "name": "Google DNS (HTTPS)",     "address": "8.8.8.8:443"        },
    { "name": "Cloudflare DNS (HTTPS)", "address": "1.1.1.1:443"        },
    { "name": "OpenDNS (HTTPS)",        "address": "208.67.222.222:443" },
    { "name": "Router Local",           "address": "192.168.1.1:80"     }
  ],
  "settings": {
    "interval_seconds": 30,
    "timeout_seconds":  5,
    "max_retries":      3
  }
}
```

> **Tip**: El campo `address` soporta `host:port` o solo `host` (asume `:443` por defecto).
> Usa `:443` (HTTPS) para servidores DNS públicos, `:80` para routers con interfaz web.

---

## 🏢 Enterprise Value

### Por qué NetPulse en entornos reales

| Escenario | Solución NetPulse |
|---|---|
| **Redundancia de ISP** | Monitorea tu enlace primario y el de backup — alerta en segundos si uno cae. |
| **SLA de proveedores** | Logs timestamped de latencia para disputar cortes con el proveedor. |
| **NOC sin agentes** | Binario estático compilado — corre sin Docker, sin dependencias externas. |
| **Integración DevOps** | Webhook a n8n/Zapier dispara runbooks automáticos al detectar caídas. |
| **Eficiencia de CPU** | `time.Ticker` + goroutines: N targets en paralelo en milisegundos, duerme el resto del intervalo. |

### Payload webhook (integración REST)

```json
{
  "event":       "alert",
  "target_name": "Router Local",
  "address":     "192.168.1.1:80",
  "old_status":  "UP",
  "new_status":  "DOWN",
  "latency_ms":  0,
  "timestamp":   "2026-09-12T22:30:00Z"
}
```

---

## 🧠 Decisiones de Diseño

### ¿Por qué channels y no mutex?

```go
// ❌ Con mutex — propenso a olvidar Lock/Unlock, difícil de razonar
var mu sync.Mutex
var all []PingResult
go func() {
    mu.Lock()
    all = append(all, result)
    mu.Unlock()
}()

// ✅ Con channel — idiomático, seguro, el tipo garantiza la sincronización
results := make(chan PingResult, targetCount)
go func(tgt Target) {
    defer wg.Done()
    results <- pingTarget(tgt, timeout)
}(target)
```

### ¿Por qué Ticker y no Sleep?

```
time.Sleep(30s)     → duerme 30s DESPUÉS de que el ciclo terminó.
                      Si el ciclo tardó 8s → próximo ciclo a los 38s.

time.NewTicker(30s) → dispara CADA 30s desde la creación.
                      Intervalo constante sin importar cuánto tarde el ciclo.
```

### Máquina de estados del Notifier

```
Estado previo  │  Estado actual  │  ¿Alerta?
───────────────┼─────────────────┼──────────────────────────────
(sin estado)   │  UP             │  No  (inicio limpio)
(sin estado)   │  DOWN/TIMEOUT   │  Sí  (caída detectada)
UP             │  UP             │  No  (sin cambio)
UP             │  DOWN/TIMEOUT   │  Sí  (nueva caída)    🔴
DOWN/TIMEOUT   │  UP             │  Sí  (recuperación)   🟢
DOWN/TIMEOUT   │  DOWN/TIMEOUT   │  No  (ya notificado)
```

---

## 📁 Estructura del Proyecto

```
netpulse/
├── cmd/
│   └── netpulse/
│       └── main.go          # Punto de entrada: flags, ticker, render de tabla
├── config/
│   └── config.go            # Carga y validación de targets.json
├── internal/
│   ├── pinger/
│   │   ├── engine.go        # Motor concurrente: goroutines + WaitGroup + channel
│   │   ├── probe.go         # Sonda TCP individual (1 intento)
│   │   └── result.go        # Tipos: PingResult, Status
│   └── notifier/
│       └── notifier.go      # Detección de transiciones + TelegramSender + WebhookSender
├── targets.json             # Configuración de hosts a monitorear
├── go.mod
└── README.md
```

---

## 🔭 Roadmap

- [ ] Sonda ICMP (raw ping) vía `golang.org/x/net/icmp`
- [ ] Exportación de métricas en formato Prometheus
- [ ] Dashboard web en tiempo real (WebSockets)
- [ ] Alertas a Slack, Discord y PagerDuty
- [ ] Persistencia de historial en SQLite

---

## 📄 Licencia

MIT © 2026 — [Try2Fun](https://github.com/Try2Fun)

---

<div align="center">
Hecho con ❤️ y Go — <em>concurrent by design, reactive by nature</em>
</div>
