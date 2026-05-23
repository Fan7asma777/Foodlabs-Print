// FoodLabs Print Agent — bridge entre foodlabs.app y la impresora térmica
// local. Bind solo a 127.0.0.1:40213 — drop-in compatible con Parzibyte.
//
// Estado: STUB M1. Los handlers responden contratos esperados por el
// frontend (Front/src/services/printService.ts) pero todavía no hablan con
// hardware real. La integración USB/winspool se agrega en milestones
// siguientes (ver README.md).
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"
)

const (
	listenAddr = "127.0.0.1:40213"
	version    = "0.1.0-stub"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/impresoras", handleListPrinters)
	mux.HandleFunc("/imprimir", handlePrint)
	mux.HandleFunc("/version", handleVersion)

	// CORS: el frontend en foodlabs.app llama a este server local. Como va de
	// origin HTTPS a 127.0.0.1, sin CORS los browsers bloquean. Solo
	// allowamos GET/POST y los headers que el cliente Supabase manda; nada
	// sensible se expone.
	handler := withCORS(mux)

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("[FoodLabs Print Agent] v%s — escuchando en %s (OS: %s)", version, listenAddr, runtime.GOOS)
	log.Printf("[FoodLabs Print Agent] STUB — handlers responden contrato pero NO imprimen todavía.")
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

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"version": version,
		"os":      runtime.GOOS,
	})
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": version,
	})
}

// handleListPrinters — drop-in compatible con Parzibyte: devuelve un array
// JSON de strings (nombres de impresoras instaladas).
//
// STUB: hoy devuelve [] — el frontend va a mostrar "no hay impresoras
// detectadas" pero NO crashea. M1 task: integrar con `winspool.EnumPrinters`
// para listar impresoras Windows reales.
func handleListPrinters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// TODO M1: enumerar impresoras Windows con winspool API.
	writeJSON(w, http.StatusOK, []string{})
}

type printRequest struct {
	Operations []map[string]any `json:"operations,omitempty"`
	Printer    string           `json:"impresora,omitempty"`
	Text       string           `json:"texto,omitempty"`
}

func handlePrint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req printRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid JSON: %v", err)})
		return
	}
	// TODO M1: encode ESC/POS y enviar a impresora USB/red. Por ahora
	// devolvemos ok=false con razón explícita para que el frontend pueda
	// distinguir "stub" de "error real".
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"ok":        false,
		"stub":      true,
		"reason":    "FoodLabs Print Agent v0.1 STUB — falta integración con impresora real (M1)",
		"received":  req,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
