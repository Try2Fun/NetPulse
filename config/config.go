// Package config handles loading and parsing the NetPulse configuration file.
// It reads targets.json and exposes strongly-typed structs for use across the app.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Target representa un host individual que NetPulse va a monitorear.
// Los campos usan json tags para mapear exactamente las claves del JSON.
type Target struct {
	Name    string `json:"name"`    // Nombre descriptivo, ej: "Router CANTV"
	Address string `json:"address"` // IP o hostname, ej: "192.168.1.1"
}

// Settings contiene la configuración de comportamiento global del motor.
type Settings struct {
	IntervalSeconds int `json:"interval_seconds"` // Cada cuántos segundos hacer ping
	TimeoutSeconds  int `json:"timeout_seconds"`  // Tiempo máximo de espera por respuesta
	MaxRetries      int `json:"max_retries"`      // Intentos antes de disparar alerta
}

// Config es la estructura raíz que representa el archivo targets.json completo.
// Actúa como el contrato de datos entre el archivo JSON y el código Go.
type Config struct {
	Targets  []Target `json:"targets"`  // Slice (lista) de hosts a monitorear
	Settings Settings `json:"settings"` // Configuración global del motor
}

// Load lee y parsea el archivo de configuración desde la ruta indicada.
//
// Retorna (*Config, error) — el patrón idiomático de Go para manejo de errores.
// El caller decide qué hacer con el error; no usamos panic() en producción.
//
// Ejemplo de uso:
//
//	cfg, err := config.Load("targets.json")
//	if err != nil {
//	    log.Fatalf("no se pudo cargar config: %v", err)
//	}
func Load(filePath string) (*Config, error) {
	// os.ReadFile lee el archivo completo en memoria como []byte.
	// Si el archivo no existe o hay un error de permisos, retorna error aquí.
	data, err := os.ReadFile(filePath)
	if err != nil {
		// fmt.Errorf con %w "envuelve" el error original.
		// Esto permite al caller usar errors.Is() o errors.As() si lo necesita.
		return nil, fmt.Errorf("error leyendo archivo de configuración '%s': %w", filePath, err)
	}

	// Validación básica: si el archivo está vacío, fallamos rápido con un mensaje claro.
	if len(data) == 0 {
		return nil, fmt.Errorf("el archivo de configuración '%s' está vacío", filePath)
	}

	// Declaramos una variable del tipo Config donde json.Unmarshal volcará los datos.
	var cfg Config

	// json.Unmarshal convierte los bytes JSON en la struct Config.
	// Go usa reflexión internamente para mapear las claves JSON a los campos via json tags.
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error parseando JSON en '%s': %w", filePath, err)
	}

	// Validación de negocio: aseguramos que el archivo tiene al menos un target.
	// Un archivo JSON válido pero sin targets es un error de configuración, no de parseo.
	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("la configuración no contiene ningún target en '%s'", filePath)
	}

	// Retornamos un puntero a la struct. Usar puntero evita copiar toda la struct
	// y permite que el caller detecte nil (aunque aquí nunca retornamos nil sin error).
	return &cfg, nil
}
