// Tray icon en la barra de tareas (M3). Permite al cajero ver que el agent
// está corriendo + cerrarlo desde click derecho sin matar proceso a la
// fuerza.
//
// Library: github.com/getlantern/systray (Go puro + cgo en bg threads).
// Funciona en Windows, Mac, Linux (con libappindicator en Linux).
//
// Cuando se ejecuta sin GUI (servidor Linux, container), systray.Run no
// arranca y el agent termina inmediatamente. Por ahora M3 solo apunta a
// Windows interactive desktop. El binary corre OK también si lo dispara
// la tarea programada NSIS (sesión user).
package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/getlantern/systray"
)

func runTray(srv *http.Server) {
	onReady := func() {
		systray.SetTitle("FoodLabs Print Agent")
		systray.SetTooltip("FoodLabs Print Agent — escuchando en 127.0.0.1:40213")
		// Icon embebido (PNG 16x16). En M3 usamos un placeholder simple;
		// M3.1 reemplazará con el logo FoodLabs.
		systray.SetIcon(trayIconBytes())

		// Items de menú: solo lectura (status) + acciones (abrir logs, salir)
		mStatus := systray.AddMenuItem("● Corriendo en :40213", "Estado del agent")
		mStatus.Disable()
		systray.AddSeparator()
		mVersion := systray.AddMenuItem("Versión "+version, "Versión del agent")
		mVersion.Disable()
		systray.AddSeparator()
		mLogs := systray.AddMenuItem("Abrir carpeta de logs", "Ver logs del agent")
		mTest := systray.AddMenuItem("Test: ping /health", "Verificar que el agent responde")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Salir", "Detener FoodLabs Print Agent")

		go func() {
			for {
				select {
				case <-mLogs.ClickedCh:
					openLogsFolder()
				case <-mTest.ClickedCh:
					testHealth()
				case <-mQuit.ClickedCh:
					log.Printf("Quit desde tray icon")
					systray.Quit()
					return
				}
			}
		}()
	}

	onExit := func() {
		log.Printf("Cerrando HTTP server…")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		log.Printf("FoodLabs Print Agent terminado.")
	}

	systray.Run(onReady, onExit)
}

// openLogsFolder abre el explorador en la carpeta de logs. Útil para que
// el cajero copie logs y nos los mande cuando algo falla.
func openLogsFolder() {
	dir, err := logsDir()
	if err != nil {
		log.Printf("openLogsFolder: %v", err)
		return
	}
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("explorer", dir).Start()
	case "darwin":
		_ = exec.Command("open", dir).Start()
	default:
		_ = exec.Command("xdg-open", dir).Start()
	}
}

// testHealth hace ping a /health del propio agent para confirmar que está
// respondiendo (útil cuando el cajero duda que esté arriba).
func testHealth() {
	client := &http.Client{Timeout: 2 * time.Second}
	res, err := client.Get("http://" + listenAddr + "/health")
	if err != nil {
		log.Printf("test /health falló: %v", err)
		return
	}
	defer res.Body.Close()
	log.Printf("test /health OK (%s)", res.Status)
}

// iconBytes cachea el PNG generado al primer acceso. Generar runtime
// garantiza bytes válidos (vs hardcoded que en v0.3.0 quedaron malformados
// y systray.SetIcon falló con "Unable to set icon: The operation completed
// successfully" — error confuso de Windows API ante PNG inválido).
var iconBytes []byte

// trayIconBytes devuelve un PNG 16x16 verde Foodlabs (#059669) generado
// con image/png stdlib. Sin dependencias externas, bytes válidos garantizados.
// M3.2 reemplazará con el logo Foodlabs real (//go:embed icon.ico).
func trayIconBytes() []byte {
	if iconBytes != nil {
		return iconBytes
	}
	const size = 16
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	// Verde Foodlabs (Tailwind emerald-600): #059669
	fg := color.RGBA{R: 0x05, G: 0x96, B: 0x69, A: 0xFF}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Bordes redondeados sutiles: las esquinas (2x2) quedan transparentes
			if (x < 2 || x >= size-2) && (y < 2 || y >= size-2) {
				continue
			}
			img.Set(x, y, fg)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Printf("trayIconBytes: png encode falló: %v", err)
		return nil
	}
	iconBytes = buf.Bytes()
	return iconBytes
}
