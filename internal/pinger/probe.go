package pinger

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// pingOnce realiza exactamente un intento TCP al address dado con el timeout indicado.
//
// # Formato de address
//
// El campo address del targets.json puede tener dos formatos:
//   - "host:port"  → se usa directamente (ej: "8.8.8.8:443")
//   - "host"       → se asume puerto :443 por defecto (HTTPS)
//
// # ¿Por qué TCP y no ICMP (ping real)?
//
// El ICMP "ping" real requiere privilegios de administrador/root en la mayoría
// de sistemas operativos modernos. Una conexión TCP es suficiente
// para verificar si un host está "vivo" en la red y es ejecutable sin privilegios.
//
// En producción se podría usar la librería "github.com/go-ping/ping" para ICMP,
// pero la stdlib de Go es suficiente para este motor.
//
// Parámetros:
//   - address: IP o hostname del target, con o sin puerto (ej: "8.8.8.8:443" o "8.8.8.8")
//   - timeout: tiempo máximo de espera (viene de config.Settings.TimeoutSeconds)
//
// Retorna:
//   - latency: duración de la conexión TCP (0 si falló)
//   - err: nil si la conexión tuvo éxito, error descriptivo si falló
func pingOnce(address string, timeout time.Duration) (latency time.Duration, err error) {
	// Resolvemos el target completo (host:port).
	// Si el address ya tiene puerto (contiene ":"), lo usamos directamente.
	// Si no, añadimos :443 (HTTPS) como puerto por defecto.
	target := resolveAddress(address)

	// Registramos el tiempo de inicio ANTES de intentar la conexión.
	// time.Now() usa el reloj monotónico del OS — no se ve afectado por cambios de hora.
	start := time.Now()

	// net.DialTimeout intenta abrir una conexión TCP.
	// Bloqueará el goroutine actual por un máximo de `timeout`.
	// Si tiene éxito, `conn` es una conexión abierta que DEBEMOS cerrar.
	conn, err := net.DialTimeout("tcp", target, timeout)

	// time.Since(start) calcula cuánto tiempo pasó desde `start` hasta ahora.
	// Lo calculamos aquí, inmediatamente después del Dial, para máxima precisión.
	latency = time.Since(start)

	if err != nil {
		// No modificamos `latency` al fallar — el caller puede ignorarla cuando err != nil.
		return latency, fmt.Errorf("conexión TCP fallida a '%s': %w", target, err)
	}

	// IMPORTANTE: Siempre cerrar conexiones de red.
	// defer garantiza que conn.Close() se llame cuando pingOnce retorne,
	// incluso si hay múltiples puntos de retorno en la función.
	defer conn.Close()

	return latency, nil
}

// resolveAddress determina el address TCP completo (host:port) a conectar.
//
// Reglas:
//   - Si el address contiene ":" → ya tiene puerto, se usa tal cual.
//   - Si no                      → se añade ":443" como defecto (HTTPS/TLS).
//
// net.JoinHostPort maneja correctamente IPv6 entre corchetes (ej: [::1]:443).
func resolveAddress(address string) string {
	// strings.Contains(address, ":") detecta si ya hay un puerto.
	// Nota: IPv6 sin puerto también contiene ":", pero en ese caso el usuario
	// debería usar la notación "[::1]:443". Esta es una heurística simple y práctica.
	if strings.Contains(address, ":") {
		return address
	}
	// Sin puerto → asumimos :443 (HTTPS). Es el puerto más probablemente
	// abierto en servidores públicos sin requerir privilegios de root.
	return net.JoinHostPort(address, "443")
}
