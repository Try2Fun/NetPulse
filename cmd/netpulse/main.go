// Comando netpulse — punto de entrada del monitor de red.
//
// # Responsabilidades de main.go
//
//  1. Cargar la configuración (config.Load).
//  2. Construir el Engine (pinger.New) y el Notifier (notifier.New).
//  3. Arrancar el servidor web en background si -web está activo.
//  4. Ejecutar el primer ciclo inmediatamente al arrancar.
//  5. Entrar en el loop de time.Ticker para ciclos periódicos.
//  6. Renderizar la tabla de resultados en consola después de cada ciclo.
//  7. Escuchar señales del OS (Ctrl+C / SIGTERM) para salida limpia.
//
// # ¿Por qué time.Ticker y no time.Sleep?
//
//	time.Sleep(30s)       → duerme 30s DESPUÉS de que el ciclo terminó.
//	                        Si el ciclo tardó 8s, el próximo ciclo empieza a los 38s.
//	time.NewTicker(30s)   → dispara CADA 30s contados desde la creación.
//	                        El intervalo es constante, independiente de cuánto tarde el ciclo.
//
// El Ticker es más correcto para monitoreo periódico.
//
// # Arquitectura de señales (graceful shutdown)
//
//	signal.NotifyContext devuelve un ctx que se cancela cuando llega SIGINT/SIGTERM.
//	El select en el loop escucha tanto <-ticker.C (nuevo ciclo) como <-ctx.Done() (salida).
//	Esto evita tener que matar el proceso con kill -9.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tuusuario/netpulse/config"
	"github.com/tuusuario/netpulse/internal/discovery"
	"github.com/tuusuario/netpulse/internal/hub"
	"github.com/tuusuario/netpulse/internal/notifier"
	"github.com/tuusuario/netpulse/internal/pinger"
	"github.com/tuusuario/netpulse/internal/server"
)

//go:embed web
var webFiles embed.FS

func main() {
	// =========================================================================
	// 1. FLAGS DE LÍNEA DE COMANDOS
	// =========================================================================
	configPath := flag.String("config", "targets.json", "Ruta al archivo de configuración JSON")
	scanNetwork := flag.Bool("scan", false, "Escanear automáticamente la red local para descubrir dispositivos vivos")
	saveScan := flag.Bool("save", false, "Guardar los dispositivos descubiertos en el archivo targets.json")
	telegramToken := flag.String("telegram-token", "", "Token del bot de Telegram (opcional)")
	telegramChat := flag.String("telegram-chat", "", "Chat ID de Telegram destino (opcional)")
	webhookURL := flag.String("webhook-url", "", "URL de webhook HTTP para alertas (opcional)")
	webEnabled := flag.Bool("web", false, "Activar el dashboard web en tiempo real")
	webPort := flag.Int("port", 8080, "Puerto para el dashboard web")
	flag.Parse()

	// =========================================================================
	// 2. CARGAR O INICIALIZAR CONFIGURACIÓN
	// =========================================================================
	var cfg *config.Config
	var err error

	cfg, err = config.Load(*configPath)
	if err != nil {
		if *scanNetwork {
			cfg = &config.Config{
				Targets: []config.Target{},
				Settings: config.Settings{
					IntervalSeconds: 30,
					TimeoutSeconds:  5,
					MaxRetries:      3,
				},
			}
		} else {
			log.Fatalf("❌ Error cargando configuración: %v (usa -scan para descubrir la red automáticamente)", err)
		}
	}

	// =========================================================================
	// 2.1. AUTO-DESCUBRIMIENTO DE RED LOCAL (LAN DISCOVERY)
	// =========================================================================
	if *scanNetwork {
		fmt.Println("\n🔍 [AUTO-DISCOVERY] Escaneando subred local con 254 sondas concurrentes...")
		startScan := time.Now()
		devices, err := discovery.ScanSubnet()
		if err != nil {
			log.Printf("⚠️  Error durante el auto-descubrimiento: %v", err)
		} else {
			scanDuration := time.Since(startScan).Round(time.Millisecond)
			fmt.Printf("✨ [AUTO-DISCOVERY] ¡Escaneo completado en %s! Se detectaron %d dispositivos activos:\n",
				scanDuration, len(devices))

			existing := make(map[string]bool)
			for _, t := range cfg.Targets {
				cleanAddr := strings.Split(t.Address, ":")[0]
				existing[cleanAddr] = true
			}

			newCount := 0
			for _, d := range devices {
				vendorTag := ""
				if d.Vendor != "" {
					vendorTag = fmt.Sprintf(" [%s]", d.Vendor)
				}
				fmt.Printf("   ├─ 📡 IP: %-15s │ MAC: %s%s │ %s\n",
					d.IP, d.MAC, vendorTag, d.DisplayName())

				if !existing[d.IP] {
					cfg.Targets = append(cfg.Targets, config.Target{
						Name:    d.DisplayName(),
						Address: d.IP,
					})
					existing[d.IP] = true
					newCount++
				}
			}

			fmt.Printf("   └─ Se incorporaron %d dispositivos nuevos al monitoreo.\n\n", newCount)

			if *saveScan {
				if err := cfg.Save(*configPath); err != nil {
					log.Printf("⚠️  No se pudo guardar la configuración: %v", err)
				} else {
					log.Printf("💾 Configuración actualizada y guardada en '%s'", *configPath)
				}
			}
		}

		// Iniciar escáner continuo en segundo plano cada 60 segundos
		go func() {
			discoveryTicker := time.NewTicker(60 * time.Second)
			defer discoveryTicker.Stop()
			for {
				select {
				case <-discoveryTicker.C:
					log.Println("🔍 [AUTO-DISCOVERY] Escaneo de fondo iniciado...")
					devices, err := discovery.ScanSubnet()
					if err != nil {
						continue
					}

					cfg.TargetsMu.Lock()
					existing := make(map[string]bool)
					for _, t := range cfg.Targets {
						cleanAddr := strings.Split(t.Address, ":")[0]
						existing[cleanAddr] = true
					}

					var added int
					for _, d := range devices {
						if !existing[d.IP] {
							cfg.Targets = append(cfg.Targets, config.Target{
								Name:    d.DisplayName(),
								Address: d.IP,
							})
							added++
							// Podríamos forzar un guardado inmediato si -save está activo:
							if *saveScan {
								cfg.Save(*configPath)
							}
						}
					}
					cfg.TargetsMu.Unlock()

					if added > 0 {
						log.Printf("✨ [AUTO-DISCOVERY] %d nuevos dispositivos añadidos al monitoreo", added)
					}
				}
			}
		}()
	}

	log.Printf("✅ Configuración lista: %d targets a monitorear, intervalo=%ds",
		len(cfg.Targets), cfg.Settings.IntervalSeconds)

	// =========================================================================
	// 3. CONSTRUIR SENDERS Y NOTIFIER
	// =========================================================================
	var senders []notifier.Sender

	if *telegramToken != "" && *telegramChat != "" {
		senders = append(senders, notifier.NewTelegramSender(*telegramToken, *telegramChat))
		log.Printf("📱 Notificaciones Telegram activadas (chat: %s)", *telegramChat)
	}

	if *webhookURL != "" {
		senders = append(senders, notifier.NewWebhookSender(*webhookURL))
		log.Printf("🔗 Notificaciones Webhook activadas (%s)", *webhookURL)
	}

	if len(senders) == 0 {
		log.Println("⚠️  Sin notificadores configurados. Modo solo-consola activo.")
		log.Println("   Usa -telegram-token/-telegram-chat o -webhook-url para activar alertas.")
	}

	n := notifier.New(senders...)

	// =========================================================================
	// 4. CONSTRUIR EL ENGINE
	// =========================================================================
	engine := pinger.New(cfg)

	// =========================================================================
	// 4.5. SERVIDOR WEB (OPTIONAL)
	// =========================================================================
	var srv *server.Server
	if *webEnabled {
		h := hub.New()
		srv = server.New(h, cfg, *configPath)

		// Extraemos el sub-filesystem "web" del embed
		webFS, fsErr := fs.Sub(webFiles, "web")
		if fsErr != nil {
			log.Fatalf("❌ No se pudo leer el directorio web embebido: %v", fsErr)
		}

		go func() {
			log.Printf("🌐 Dashboard web iniciado en http://localhost:%d", *webPort)
			if startErr := srv.Start(*webPort, webFS); startErr != nil {
				log.Printf("❌ Error en servidor web: %v", startErr)
			}
		}()
	}

	// =========================================================================
	// 5. GRACEFUL SHUTDOWN CON CONTEXTO DE SEÑALES
	// =========================================================================
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// =========================================================================
	// 6. PRIMER CICLO INMEDIATO
	// =========================================================================
	log.Println("🚀 NetPulse iniciado. Ejecutando primer ciclo...")
	runCycle(engine, n, srv)

	// =========================================================================
	// 7. LOOP DE MONITOREO CON time.Ticker
	// =========================================================================
	interval := time.Duration(cfg.Settings.IntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("⏱  Próximo ciclo en %s. Presiona Ctrl+C para salir.\n", interval)

	for {
		select {
		case t := <-ticker.C:
			log.Printf("🔄 Ciclo iniciado a las %s", t.Format("15:04:05"))
			runCycle(engine, n, srv)

		case <-ctx.Done():
			fmt.Println("\n\n👋 Saliendo de NetPulse. ¡Hasta pronto!")
			return
		}
	}
}

// =============================================================================
// CICLO DE MONITOREO
// =============================================================================

// runCycle ejecuta un ciclo completo: ping a todos los targets → notificación → render → broadcast.
func runCycle(engine *pinger.Engine, n *notifier.Notifier, srv *server.Server) {
	start := time.Now()

	results := engine.Run()

	elapsed := time.Since(start)

	n.Process(results)

	renderTable(results, elapsed)

	// Si el servidor web está activo, transmitimos los resultados
	if srv != nil {
		srv.BroadcastCycle(results, elapsed)
	}
}

// =============================================================================
// RENDERIZADO DE CONSOLA
// =============================================================================

// renderTable imprime una tabla ASCII con los resultados del ciclo actual.
//
// Ejemplo de salida:
//
//	╔══════════════════════════════════════════════════════════════╗
//	║  NetPulse  │  12/09/2026 18:30:00  │  ciclo en 4.23s       ║
//	╠══════════════════╦══════════════╦══════════╦════════════════╣
//	║ TARGET           ║ DIRECCIÓN    ║ ESTADO   ║ LATENCIA       ║
//	╠══════════════════╬══════════════╬══════════╬════════════════╣
//	║ Google DNS       ║ 8.8.8.8      ║ 🟢 UP    ║ 14ms           ║
//	║ Cloudflare DNS   ║ 1.1.1.1      ║ 🟢 UP    ║ 12ms           ║
//	║ Router CANTV     ║ 192.168.1.1  ║ 🔴 DOWN  ║ —              ║
//	╚══════════════════╩══════════════╩══════════╩════════════════╝
func renderTable(results []pinger.PingResult, elapsed time.Duration) {
	now := time.Now().Format("02/01/2006 15:04:05")

	header := fmt.Sprintf("  NetPulse  │  %s  │  ciclo en %.2fs  ",
		now, elapsed.Seconds())
	width := max(len(header)+2, 64)
	line := strings.Repeat("═", width)

	fmt.Println()
	fmt.Printf("╔%s╗\n", line)
	fmt.Printf("║ %-*s║\n", width, header)
	fmt.Printf("╠%s╣\n", line)

	fmt.Printf("║  %-20s  ║  %-15s  ║  %-12s  ║  %-12s  ║\n",
		"TARGET", "DIRECCIÓN", "ESTADO", "LATENCIA")
	fmt.Printf("╠%s╣\n", line)

	for _, r := range results {
		statusStr := formatStatus(r.Status)
		latencyStr := "—"
		if r.Status == pinger.StatusUp {
			latencyStr = r.Latency.Round(time.Millisecond).String()
		}

		fmt.Printf("║  %-20s  ║  %-15s  ║  %-12s  ║  %-12s  ║\n",
			truncate(r.TargetName, 20),
			truncate(r.Address, 15),
			statusStr,
			latencyStr,
		)
	}

	fmt.Printf("╚%s╝\n", line)
	fmt.Println()
}

func formatStatus(s pinger.Status) string {
	switch s {
	case pinger.StatusUp:
		return "🟢 UP"
	case pinger.StatusDown:
		return "🔴 DOWN"
	case pinger.StatusTimeout:
		return "🟡 TIMEOUT"
	default:
		return string(s)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
