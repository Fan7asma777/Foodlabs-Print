// FoodLabs Print Agent — bridge entre foodlabs.app y la impresora térmica
// local. Bind solo a 127.0.0.1:40213 — drop-in compatible con Parzibyte.
//
// M1 features:
//   - GET /impresoras        Lista impresoras Windows (winspool)
//   - POST /imprimir         Recibe texto + flags, encode ESC/POS y envía
//                            con corte automático + beep (opcional)
//   - GET /health, /version  Stub para monitoring + auto-update futuro
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"
)

const (
	listenAddr = "127.0.0.1:40213"
	version    = "0.1.0"
)

func main() {
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
	log.Printf("[FoodLabs Print Agent] foodlabs.app ya puede detectar este agent")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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

// handleListPrinters — devuelve array de nombres de impresoras instaladas.
// Compatible con la API de Parzibyte que el frontend ya usa.
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

// printRequest — payload del POST /imprimir.
//
// El frontend manda `texto` con el ticket en texto plano. Las `lines`
// adicionales permiten flags por línea (negrita, doble alto, etc.) — futuro.
//
// `cut: true` corta el papel al final (feature pedida por user M1).
// `beep: true` hace que la impresora suene 3 veces antes de imprimir
// (alerta cocina, feature pedida por user M1).
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

	// Construir el buffer ESC/POS final con flags M1.
	var buf bytes.Buffer
	if req.Beep {
		// ESC B n1 n2 = beep n1 veces durante n2*50ms (ESC/POS standard)
		// 3 beeps × 100ms = 300ms total → suficiente para alertar cocina
		buf.Write([]byte{0x1B, 0x42, 3, 2})
	}
	// ESC @ = initialize printer (reset margin, font, etc)
	buf.Write([]byte{0x1B, 0x40})
	// Texto del ticket — el frontend YA construye el layout
	buf.WriteString(req.Texto)
	// 4 line feeds para que el papel salga lo suficiente antes del corte
	buf.WriteString("\n\n\n\n")
	if req.Cut {
		// GS V 1 = partial cut (deja una pequeña pestañita para que no caiga)
		// Compatible con la mayoría de impresoras térmicas Epson/Star/Bixolon
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
		"ok":       true,
		"impresora": req.Printer,
		"bytes":    buf.Len(),
		"cut":      req.Cut,
		"beep":     req.Beep,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// listPrinters y sendToPrinter están en archivos OS-specific:
//
//   printer_windows.go   - usa winspool.dll via github.com/alexbrainman/printer
//   printer_other.go     - stub para Linux/Mac (devuelve error claro)
//
// Build constraints en cada archivo. M1 = solo Windows.
//
//   go build -o print-agent.exe ./cmd/agent
//
// Cross-compile desde Mac/Linux:
//   GOOS=windows GOARCH=amd64 go build -o print-agent.exe ./cmd/agent
