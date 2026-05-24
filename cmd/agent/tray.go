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
	_ "embed"
	"encoding/binary"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/getlantern/systray"
)

// Logo Foodlabs (matraz colorido) — copia exacta del logo oficial en
// Front/public/logo-foodlabs.png. Windows scale-downea para el tray a
// 16x16 con anti-aliasing decente.
//
//go:embed assets/foodlabs-logo.png
var foodlabsLogoPNG []byte

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

// trayIconBytes devuelve el logo Foodlabs listo para systray.SetIcon.
//
// getlantern/systray en Windows requiere bytes ICO (NO PNG). Si le pasamos
// PNG directo, falla silencioso ("Unable to set icon: The operation
// completed successfully") y la barra de tareas queda sin icono.
//
// Solución: envolver el PNG embebido en un contenedor ICO de 1 imagen.
// Windows Vista+ acepta PNG embebido dentro de ICO vía
// CreateIconFromResourceEx. Mac/Linux también lo aceptan (compatible).
func trayIconBytes() []byte {
	return wrapPNGAsICO(foodlabsLogoPNG)
}

// wrapPNGAsICO arma un ICO de 1 imagen con el PNG dentro. Header ICONDIR
// (6 bytes) + ICONDIRENTRY (16 bytes) = 22 bytes de overhead, después el
// PNG entero. Reportamos 256x256 en el header (campo 0 = 256 según spec
// ICO) y dejamos que Windows scale-downee al tamaño del tray (16x16).
func wrapPNGAsICO(pngData []byte) []byte {
	var buf bytes.Buffer
	// ICONDIR (6 bytes)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // type 1 = ICON
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // image count
	// ICONDIRENTRY (16 bytes)
	buf.WriteByte(0)                                         // width  (0 = 256)
	buf.WriteByte(0)                                         // height (0 = 256)
	buf.WriteByte(0)                                         // colors in palette
	buf.WriteByte(0)                                         // reserved
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))   // color planes
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32))  // bits per pixel
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(pngData))) // image size
	_ = binary.Write(&buf, binary.LittleEndian, uint32(22))  // offset to PNG
	buf.Write(pngData)
	return buf.Bytes()
}
