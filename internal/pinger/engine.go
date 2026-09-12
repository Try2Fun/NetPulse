package pinger

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/tuusuario/netpulse/config"
)

// Engine es el motor de ping concurrente. Orquesta workers y recolecta resultados.
//
// Separamos Engine como struct para que en el futuro pueda tener estado propio
// (ej: historial de resultados, métricas acumuladas) sin cambiar la firma pública.
type Engine struct {
	cfg *config.Config
}

// New crea una instancia del Engine con la configuración cargada.
// Seguimos el patrón constructor de Go: función New() que retorna el tipo.
func New(cfg *config.Config) *Engine {
	return &Engine{cfg: cfg}
}

// Run ejecuta un ciclo completo de ping a todos los targets de forma concurrente
// y bloquea hasta que TODOS los resultados hayan sido recolectados.
//
// # Flujo de concurrencia paso a paso:
//
//  1. Creamos un channel `results` donde cada goroutine depositará su PingResult.
//  2. Lanzamos N goroutines (una por target) con `go func()`.
//  3. Usamos sync.WaitGroup para saber cuándo terminaron TODAS las goroutines.
//  4. Una goroutine separada cierra el channel cuando todas terminaron.
//  5. El loop `for result := range results` recolecta cada resultado.
//
// ¿Por qué no usar un mutex y un slice compartido?
//
//	// VERSIÓN CON MUTEX (más propensa a errores):
//	var mu sync.Mutex
//	var all []PingResult
//	go func() {
//	    mu.Lock()
//	    all = append(all, result)  // fácil olvidar el Lock/Unlock
//	    mu.Unlock()
//	}()
//
// El channel es más idiomático en Go: "No comuniques compartiendo memoria;
// comparte memoria comunicándote." — Rob Pike, creador de Go.
func (e *Engine) Run() []PingResult {
	e.cfg.TargetsMu.RLock()
	targetCount := len(e.cfg.Targets)
	targetsCopy := make([]config.Target, targetCount)
	copy(targetsCopy, e.cfg.Targets)
	e.cfg.TargetsMu.RUnlock()

	timeout := time.Duration(e.cfg.Settings.TimeoutSeconds) * time.Second

	// =========================================================================
	// PASO 1: Crear el channel con buffer
	// =========================================================================
	//
	// make(chan PingResult, N) crea un channel BUFFERIZADO con capacidad N.
	//
	// ┌─────────────────────────────────────────────────────────┐
	// │  Channel sin buffer (N=0):                              │
	// │    goroutine A envía → BLOQUEA hasta que B reciba       │
	// │  Channel con buffer (N=targetCount):                    │
	// │    goroutine A envía → NO BLOQUEA (hay espacio)         │
	// │    goroutine B puede leer cuando quiera                 │
	// └─────────────────────────────────────────────────────────┘
	//
	// Usar buffer = targetCount es la decisión clave aquí:
	// cada goroutine worker puede depositar su resultado y terminar
	// SIN esperar a que el recolector lea. Esto maximiza el paralelismo.
	results := make(chan PingResult, targetCount)

	// =========================================================================
	// PASO 2: WaitGroup — contador atómico de goroutines activas
	// =========================================================================
	//
	// sync.WaitGroup funciona como un contador thread-safe:
	//   wg.Add(N)  → suma N al contador
	//   wg.Done()  → resta 1 al contador (llamado por cada goroutine al terminar)
	//   wg.Wait()  → BLOQUEA hasta que el contador llegue a 0
	//
	// REGLA CRÍTICA: Add() ANTES de lanzar la goroutine.
	// Si hiciéramos wg.Add() DENTRO de la goroutine, podría ejecutarse wg.Wait()
	// antes de que la goroutine llame Add(), causando una condición de carrera.
	var wg sync.WaitGroup

	// =========================================================================
	// PASO 3: Lanzar N goroutines worker (una por target)
	// =========================================================================
	for _, target := range targetsCopy {
		wg.Add(1) // Incrementamos ANTES de `go`

		// Captura de variable en loop: asignamos a variable local `t`.
		//
		// TRAMPA CLÁSICA DE GO (pre-1.22):
		//   for _, target := range ... {
		//       go func() {
		//           fmt.Println(target.Name) // ← BUG: todas las goroutines
		//       }()                          //   ven el mismo `target` (la última iteración)
		//   }
		//
		// SOLUCIÓN: Pasamos el target como argumento a la función anónima.
		// Cada goroutine recibe su propia COPIA del valor en el momento del lanzamiento.
		// (En Go 1.22+ el loop crea una nueva variable por iteración, pero pasar
		// como argumento sigue siendo más explícito y compatible.)
		t := target // copia local de esta iteración

		go func(tgt config.Target) {
			// defer wg.Done() garantiza que Done() se llame al retornar,
			// incluso si hay un panic() inesperado en pingOnce.
			defer wg.Done()

			result := e.pingTarget(tgt, timeout)

			// Enviamos el resultado al channel.
			// Con buffer = targetCount, esta operación NO bloquea.
			// Sin buffer, bloquearía hasta que el recolector lea — menos eficiente.
			results <- result

		}(t) // ← t se pasa como argumento aquí, creando una copia en el stack de la goroutine
	}

	// =========================================================================
	// PASO 4: Goroutine cerradora — el patrón "closer"
	// =========================================================================
	//
	// Necesitamos cerrar el channel para que el `range` del paso 5 sepa cuándo parar.
	// Un channel abierto con `range` nunca termina → deadlock.
	//
	// ¿Por qué otra goroutine? Porque wg.Wait() bloquea, y si lo llamáramos
	// en la goroutine principal antes del `range`, nunca llegaríamos al range.
	//
	// Diagrama de flujo:
	//
	//   main goroutine              goroutine "closer"
	//   ─────────────               ─────────────────
	//   lanza workers  ─────────►  wg.Wait() [bloqueado]
	//   lanza closer               ...workers terminan y llaman Done()
	//   range results  ◄─────────  close(results) [cuando wg llega a 0]
	//   (procesa cada resultado)
	//   (range termina al cerrar el channel)
	go func() {
		wg.Wait()     // Espera a que todos los workers llamen Done()
		close(results) // Cierra el channel → el range del paso 5 terminará
	}()

	// =========================================================================
	// PASO 5: Recolectar resultados con `range` sobre el channel
	// =========================================================================
	//
	// `for result := range results` hace dos cosas automáticamente:
	//   1. Recibe cada valor del channel (bloqueando si está vacío)
	//   2. Termina el loop cuando el channel está CERRADO Y VACÍO
	//
	// Es equivalente a:
	//   for {
	//       result, ok := <-results
	//       if !ok { break } // channel cerrado y vacío
	//       collected = append(collected, result)
	//   }
	var collected []PingResult
	for result := range results {
		collected = append(collected, result)
	}

	// En este punto: todas las goroutines terminaron Y todos los resultados fueron leídos.
	return collected
}

// pingTarget ejecuta el ping a un target con reintentos según config.Settings.MaxRetries.
// Es un método privado — solo llamado por los workers dentro de Run().
func (e *Engine) pingTarget(target config.Target, timeout time.Duration) PingResult {
	maxRetries := e.cfg.Settings.MaxRetries

	var lastErr error
	var latency time.Duration

	// Bucle de reintentos: intentamos maxRetries veces antes de rendernos.
	for attempt := 1; attempt <= maxRetries; attempt++ {
		var err error
		latency, err = pingOnce(target.Address, timeout)

		if err == nil {
			// ¡Éxito! Retornamos inmediatamente sin más reintentos.
			return PingResult{
				TargetName: target.Name,
				Address:    target.Address,
				Status:     StatusUp,
				Latency:    latency,
				Timestamp:  time.Now(),
				Err:        nil,
			}
		}

		// Guardamos el último error para incluirlo en el resultado final.
		lastErr = err
		log.Printf("[INTENTO %d/%d] %s (%s): %v", attempt, maxRetries, target.Name, target.Address, err)

		// Si no es el último intento, esperamos un momento antes de reintentar.
		// Evitar bombardear un host caído con reintentos inmediatos.
		if attempt < maxRetries {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Todos los intentos fallaron. Determinamos si fue timeout u otro error.
	status := StatusDown
	if isTimeoutError(lastErr) {
		status = StatusTimeout
	}

	return PingResult{
		TargetName: target.Name,
		Address:    target.Address,
		Status:     status,
		Latency:    0,
		Timestamp:  time.Now(),
		Err:        fmt.Errorf("falló tras %d intentos: %w", maxRetries, lastErr),
	}
}

// isTimeoutError determina si un error de red es específicamente un timeout.
// Usamos la interfaz net.Error que expone el método Timeout() bool.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	// Type assertion a la interfaz net.Error.
	// `ok` es true si el error implementa net.Error.
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}
