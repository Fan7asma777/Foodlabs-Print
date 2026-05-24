// FoodLabs Print Agent — bridge entre foodlabs.app y la impresora térmica
// local. Bind solo a 127.0.0.1:40213 — drop-in compatible con Parzibyte.
//
// v0.2.0 (M2+M3):
//   - Tray icon en la barra de tareas Windows (sin ventana negra)
//   - Logs rotativos en %APPDATA%\FoodLabsPrintAgent\logs\
//   - HTTP server corre en goroutine; systray.Run mantiene proceso vivo
//
// API HTTP (puerto 40213, drop-in compatible Parzibyte):
//   GET /health, /version
//   GET /impresoras
//   POST /imprimir
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	listenAddr = "127.0.0.1:40213"
	version    = "0.3.2"
)

func main() {
	// Logs a archivo + stdout para no perder nada cuando corre sin consola
	// (caso M2 instalado vía NSIS donde no hay ventana visible).
	setupLogging()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/version", handleVersion)
	mux.HandleFunc("/impresoras", handleListPrinters)
	mux.HandleFunc("/imprimir", handlePrint)

	handler := withCORS(withLogging(mux))

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("[FoodLabs Print Agent] v%s — escuchando en http://%s (OS: %s)", version, listenAddr, runtime.GOOS)

	// HTTP server en goroutine; el proceso principal vive en el tray icon.
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	// Auto-update (M4): chequea GitHub Releases cada 6h, descarga + reinicia
	// solo si hay versión nueva. Best-effort: si falla NO rompe operación.
	startAutoUpdater()

	runTray(srv) // bloqueante hasta que el usuario haga "Salir"
}

// setupLogging configura el logger global de Go para escribir a:
//   - stdout (visible si hay consola)
//   - %APPDATA%\FoodLabsPrintAgent\logs\agent.log (siempre)
//
// Rotación manual: si el log pasa 5MB, lo renombramos a .old y empezamos
// uno nuevo. Más simple que lumberjack y sin deps extra.
func setupLogging() {
	dir, err := logsDir()
	if err != nil {
		// Sin acceso a APPDATA — caemos a stdout solamente.
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Printf("warn: no se pudo crear directorio de logs: %v", err)
		return
	}

	logPath := filepath.Join(dir, "agent.log")
	// Rotación trivial: si pesa >5MB, lo renombramos.
	if info, err := os.Stat(logPath); err == nil && info.Size() > 5*1024*1024 {
		_ = os.Rename(logPath, logPath+".old")
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Printf("warn: no se pudo abrir log file: %v", err)
		return
	}
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// logsDir devuelve %APPDATA%\FoodLabsPrintAgent\logs en Windows,
// $HOME/.foodlabs-print-agent/logs en otros OSes.
func logsDir() (string, error) {
	var base string
	if runtime.GOOS == "windows" {
		base = os.Getenv("APPDATA")
		if base == "" {
			base, _ = os.UserConfigDir()
		}
	} else {
		base, _ = os.UserHomeDir()
		base = filepath.Join(base, ".foodlabs-print-agent")
	}
	dir := filepath.Join(base, "FoodLabsPrintAgent", "logs")
	if runtime.GOOS != "windows" {
		dir = filepath.Join(base, "logs")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		// Chrome 104+ Private Network Access: requests HTTPS → 127.0.0.1
		// requieren este header sino Chrome bloquea silencioso.
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%v)", r.Method, r.URL.Path, time.Since(start))
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"version": version,
		"os":      runtime.GOOS,
	})
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": version})
}

func handleListPrinters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	printers, err := listPrinters()
	if err != nil {
		log.Printf("listPrinters error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, printers)
}

type printRequest struct {
	Printer string `json:"impresora"`
	Texto   string `json:"texto"`
	Cut     bool   `json:"cut,omitempty"`
	Beep    bool   `json:"beep,omitempty"`
}

func handlePrint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req printRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if strings.TrimSpace(req.Printer) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "campo 'impresora' requerido"})
		return
	}
	if req.Texto == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "campo 'texto' requerido"})
		return
	}

	var buf bytes.Buffer
	if req.Beep {
		// ESC B n1 n2 — 3 beeps × 100ms cada uno
		buf.Write([]byte{0x1B, 0x42, 3, 2})
	}
	// ESC @ = initialize printer
	buf.Write([]byte{0x1B, 0x40})
	buf.WriteString(req.Texto)
	buf.WriteString("\n\n\n\n")
	if req.Cut {
		// GS V 1 = partial cut
		buf.Write([]byte{0x1D, 0x56, 0x01})
	}

	if err := sendToPrinter(req.Printer, buf.Bytes()); err != nil {
		log.Printf("sendToPrinter(%q) error: %v", req.Printer, err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"impresora": req.Printer,
		"bytes":     buf.Len(),
		"cut":       req.Cut,
		"beep":      req.Beep,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
