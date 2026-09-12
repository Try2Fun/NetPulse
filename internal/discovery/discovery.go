// Package discovery implementa el auto-descubrimiento de dispositivos en la red local (LAN).
//
// Utiliza paquetes ARP concurrentes mediante llamadas a la API del sistema operativo (Win32 SendARP)
// y resolución inversa de nombres DNS/mDNS para identificar hosts vivos en la subred
// sin necesidad de permisos de Administrador ni configuración manual.
package discovery

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/tuusuario/netpulse/config"
)

var (
	modiphlpapi = syscall.NewLazyDLL("iphlpapi.dll")
	procSendARP = modiphlpapi.NewProc("SendARP")
)

// Device representa un equipo descubierto en la red local.
type Device struct {
	IP        string
	MAC       string
	Hostname  string
	IsRouter  bool
	IsSelf    bool
	Vendor    string
}

// ScanSubnet detecta la interfaz activa y escanea las 254 IPs de la subred local de forma concurrente.
// Retorna la lista de dispositivos que respondieron en la red.
func ScanSubnet() ([]Device, error) {
	localIP, err := getOutboundIP()
	if err != nil {
		return nil, fmt.Errorf("no se pudo determinar la IP local: %w", err)
	}

	// Asumimos máscara /24 típica de redes hogareñas (ej: 192.168.110.X)
	parts := strings.Split(localIP.String(), ".")
	if len(parts) != 4 {
		return nil, fmt.Errorf("formato de IP no soportado: %s", localIP.String())
	}
	subnetPrefix := fmt.Sprintf("%s.%s.%s.", parts[0], parts[1], parts[2])

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		devices []Device
	)

	// Canal con semáforo para limitar a 64 goroutines simultáneas y no saturar el driver de red
	sem := make(chan struct{}, 64)

	for i := 1; i <= 254; i++ {
		targetIP := fmt.Sprintf("%s%d", subnetPrefix, i)
		wg.Add(1)

		go func(ipStr string, hostNum int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			mac, alive := sendARP(ipStr)
			if !alive {
				// Fallback: intentar conexión TCP rápida a puertos comunes (80, 443, 53)
				alive = quickTCPCheck(ipStr)
			}

			if alive {
				isSelf := ipStr == localIP.String()
				isRouter := hostNum == 1 // Típicamente la IP .1 es el gateway

				hostname := resolveHostname(ipStr)
				vendor := detectVendor(mac)

				dev := Device{
					IP:       ipStr,
					MAC:      mac,
					Hostname: hostname,
					IsRouter: isRouter,
					IsSelf:   isSelf,
					Vendor:   vendor,
				}

				mu.Lock()
				devices = append(devices, dev)
				mu.Unlock()
			}
		}(targetIP, i)
	}

	wg.Wait()
	return devices, nil
}

// ToTargets convierte la lista de dispositivos descubiertos en targets para NetPulse.
func ToTargets(devices []Device) []config.Target {
	var targets []config.Target

	for _, d := range devices {
		name := d.DisplayName()
		address := d.IP
		// Si es el router, agregamos :80 por compatibilidad
		if d.IsRouter {
			address = d.IP + ":80"
		}
		targets = append(targets, config.Target{
			Name:    name,
			Address: address,
		})
	}

	return targets
}

// DisplayName genera un nombre amigable para mostrar en la tabla.
func (d *Device) DisplayName() string {
	if d.IsRouter {
		return "Router Gateway"
	}
	if d.IsSelf {
		return "Esta Computadora (PC)"
	}
	if d.Hostname != "" {
		return d.Hostname
	}
	if d.Vendor != "" {
		return fmt.Sprintf("Disp. %s (%s)", d.Vendor, d.IP)
	}
	return fmt.Sprintf("Dispositivo (%s)", d.IP)
}

// getOutboundIP obtiene la IP de la interfaz local que tiene salida a internet.
func getOutboundIP() (net.IP, error) {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

// sendARP realiza una llamada Win32 SendARP para verificar si una IP está activa en la LAN.
func sendARP(ipStr string) (string, bool) {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return "", false
	}

	// La API de Windows requiere la IP en formato little-endian / int32
	destIP := *(*uint32)(unsafe.Pointer(&ip[0]))

	var mac [6]byte
	macLen := uint32(len(mac))

	ret, _, _ := procSendARP.Call(
		uintptr(destIP),
		0,
		uintptr(unsafe.Pointer(&mac[0])),
		uintptr(unsafe.Pointer(&macLen)),
	)

	if ret == 0 && macLen == 6 {
		macStr := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
			mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
		return macStr, true
	}

	return "", false
}

// quickTCPCheck prueba si algún puerto estándar responde en menos de 300ms.
func quickTCPCheck(ipStr string) bool {
	ports := []string{"80", "443", "53", "8080"}
	for _, p := range ports {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ipStr, p), 250*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

// resolveHostname intenta obtener el nombre del host mediante reverse DNS.
func resolveHostname(ipStr string) string {
	names, err := net.LookupAddr(ipStr)
	if err == nil && len(names) > 0 {
		h := strings.TrimSuffix(names[0], ".")
		return h
	}
	return ""
}

// detectVendor identifica el fabricante más probable a partir de los primeros 3 bytes de la MAC (OUI).
func detectVendor(mac string) string {
	if len(mac) < 8 {
		return ""
	}
	prefix := strings.ToUpper(mac[:8])

	ouiMap := map[string]string{
		"00:50:56": "VMware",
		"08:00:27": "VirtualBox",
		"68:C8:C0": "Consola/Móvil",
		"BE:3A:CC": "Smartphone/Tablet",
		"C0:A4:76": "Router ZTE/Huawei",
		"FC:EC:DA": "Ubiquiti",
		"B8:27:EB": "Raspberry Pi",
		"DC:A6:32": "Raspberry Pi",
		"E4:5F:01": "Raspberry Pi",
		"00:1A:11": "Google",
		"3C:5A:B4": "Google Nest",
		"AC:CF:85": "Apple",
		"F0:18:98": "Apple",
		"38:F9:D3": "Apple iPhone",
		"BC:D0:74": "Apple",
		"00:04:20": "Cisco",
		"44:D9:E7": "Ubiquiti",
		"70:4F:57": "Sony PlayStation",
		"00:D9:D1": "Sony PlayStation",
		"F8:46:1C": "Sony PlayStation",
		"00:13:E8": "Intel",
		"A4:BB:6D": "Samsung",
		"50:01:D9": "Xiaomi",
		"74:23:44": "Xiaomi",
		"00:15:5D": "Microsoft Hyper-V",
	}

	for p, v := range ouiMap {
		if strings.HasPrefix(prefix, p) {
			return v
		}
	}
	return ""
}
