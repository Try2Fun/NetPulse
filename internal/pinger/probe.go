package pinger

import (
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	modiphlpapi = syscall.NewLazyDLL("iphlpapi.dll")
	procSendARP = modiphlpapi.NewProc("SendARP")
)

// pingOnce realiza un intento de sonda hacia el host dado con el timeout indicado.
//
// # Estrategia dual (ARP para LAN / TCP para WAN):
//
//   - Si el address no tiene puerto (ej: "192.168.110.30") y es una IP local,
//     utiliza sondeo ARP (Capa 2 / Data Link). Esto permite detectar consolas
//     (PlayStation, Xbox), teléfonos y televisores inteligentes sin requerir
//     que tengan puertos web abiertos ni permisos de administrador.
//
//   - Si el address tiene puerto explícito (ej: "8.8.8.8:443" o "192.168.110.1:80"),
//     utiliza conexión TCP directa (Capa 4 / Transporte).
//
// Retorna:
//   - latency: duración de la respuesta
//   - err: nil si la conexión tuvo éxito, error descriptivo si falló
func pingOnce(address string, timeout time.Duration) (latency time.Duration, err error) {
	// 1. Si no tiene puerto y es una IP válida, intentamos sondeo ARP de alta velocidad
	if !strings.Contains(address, ":") {
		if ip := net.ParseIP(address).To4(); ip != nil {
			start := time.Now()
			if probeARP(ip) {
				latency = time.Since(start)
				return latency, nil
			}
		}
	}

	// 2. Conexión TCP estándar
	target := resolveAddress(address)
	start := time.Now()

	conn, err := net.DialTimeout("tcp", target, timeout)
	latency = time.Since(start)

	if err != nil {
		return latency, fmt.Errorf("conexión TCP fallida a '%s': %w", target, err)
	}
	defer conn.Close()

	return latency, nil
}

// probeARP envía un paquete ARP mediante la API nativa de Windows (SendARP).
func probeARP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	destIP := *(*uint32)(unsafe.Pointer(&ip[0]))
	var mac [6]byte
	macLen := uint32(len(mac))

	ret, _, _ := procSendARP.Call(
		uintptr(destIP),
		0,
		uintptr(unsafe.Pointer(&mac[0])),
		uintptr(unsafe.Pointer(&macLen)),
	)
	return ret == 0 && macLen == 6
}

// resolveAddress determina el address TCP completo (host:port) a conectar.
func resolveAddress(address string) string {
	if strings.Contains(address, ":") {
		return address
	}
	return net.JoinHostPort(address, "443")
}
