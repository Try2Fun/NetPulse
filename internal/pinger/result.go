// Package pinger implementa el motor de ping concurrente de NetPulse.
//
// Arquitectura de concurrencia:
//
//	main goroutine
//	    └─► Engine.Run()
//	             ├─► goroutine #1 → ping(target[0]) → channel → collector
//	             ├─► goroutine #2 → ping(target[1]) → channel → collector
//	             └─► goroutine #N → ping(target[N]) → channel → collector
//
// El channel actúa como "tubería" segura entre productores (goroutines de ping)
// y el consumidor (goroutine recolectora). Evita condiciones de carrera (race conditions)
// sin necesidad de mutexes explícitos.
package pinger

import "time"

// Status representa el estado de salud de un target en un instante dado.
// Usar un tipo definido (no string cruda) hace el código más seguro:
// el compilador rechaza StatusUp == "arriba" por error de tipo.
type Status string

const (
	// StatusUp indica que el host respondió dentro del timeout.
	StatusUp Status = "UP"

	// StatusDown indica que el host no respondió o hubo error de red.
	StatusDown Status = "DOWN"

	// StatusTimeout indica específicamente que la operación expiró.
	// Diferenciamos timeout de "down" para poder generar alertas distintas en el futuro.
	StatusTimeout Status = "TIMEOUT"
)

// PingResult encapsula toda la información relevante de un único intento de ping.
//
// Es el tipo de dato que viaja a través del channel desde cada goroutine worker
// hasta la goroutine recolectora. Debe ser pequeño y copiable eficientemente.
//
// ¿Por qué no usar puntero (*PingResult)?
// Los channels en Go son thread-safe. Enviar una copia del struct por el channel
// es más seguro que enviar un puntero: evitamos que el worker modifique la struct
// después de haberla enviado.
type PingResult struct {
	// TargetName es el nombre descriptivo del host (ej: "Router CANTV").
	// Lo copiamos desde config.Target para que el resultado sea autocontenido.
	TargetName string

	// Address es la IP o hostname que fue pingeado.
	Address string

	// Status resume el resultado: UP, DOWN o TIMEOUT.
	Status Status

	// Latency es el tiempo que tardó en responder el host.
	// Si Status != UP, este campo no tiene significado útil (será 0).
	// time.Duration es un int64 de nanosegundos internamente — altamente preciso.
	Latency time.Duration

	// Timestamp registra exactamente cuándo se completó (o falló) el ping.
	// Usamos time.Time en lugar de int64 unix para aprovechar los métodos de formatting.
	Timestamp time.Time

	// Err captura el error de red si ocurrió uno.
	// nil cuando Status == StatusUp. Guardarlo aquí permite logging detallado.
	Err error
}
