module github.com/foodlabs/print-agent

go 1.22

// Dependencies se agregan al hacer go get cuando construyamos los handlers
// reales. Por ahora el stub usa solo stdlib (net/http, encoding/json).
//
// Próximas a sumar (M1-M2):
//   github.com/hennedo/escpos          // ESC/POS encoding
//   github.com/google/gousb            // USB raw (libusb wrapper)
//   github.com/getlantern/systray      // Tray icon Windows
//   github.com/minio/selfupdate        // Auto-update
