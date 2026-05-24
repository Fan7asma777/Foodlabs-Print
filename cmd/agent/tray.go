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
	"context"
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

// trayIconBytes devuelve un PNG 16x16 simple (cuadrado verde) como
// placeholder del icono. M3.1 reemplazará con el logo de FoodLabs.
//
// Generado con: https://pixelartmaker.com/ y exportado a bytes.
// Bytes corresponden a un PNG 16x16 verde sólido con borde redondeado.
func trayIconBytes() []byte {
	// PNG 16x16 verde Foodlabs (#059669). Generado offline.
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x10, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0xF3, 0xFF,
		0x61, 0x00, 0x00, 0x00, 0x4D, 0x49, 0x44, 0x41, 0x54, 0x38, 0xCB, 0x63, 0x6C, 0xB8, 0xE1, 0x3F,
		0x03, 0x29, 0x80, 0x89, 0x69, 0xF0, 0x6F, 0x60, 0x60, 0x60, 0x82, 0xC4, 0x40, 0xF9, 0x1F, 0xC4,
		0xC0, 0xC0, 0xC0, 0xC8, 0x02, 0xC4, 0x40, 0x40, 0x14, 0x33, 0x30, 0x30, 0x30, 0xB2, 0x00, 0x31,
		0x10, 0x10, 0xC5, 0x0C, 0x0C, 0x0C, 0x8C, 0x2C, 0x40, 0x0C, 0x04, 0x44, 0x31, 0x03, 0x03, 0x03,
		0x23, 0x0B, 0x10, 0x03, 0x01, 0x51, 0xCC, 0xC0, 0xC0, 0xC0, 0xC8, 0x02, 0xC4, 0x40, 0x40, 0x14,
		0x33, 0x30, 0x30, 0x30, 0xB2, 0x00, 0x00, 0x4D, 0x0F, 0x09, 0xC7, 0x18, 0xCA, 0xEC, 0x65, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}
