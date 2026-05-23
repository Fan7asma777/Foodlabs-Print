//go:build windows

// Integración con winspool (Windows printing API) vía
// github.com/alexbrainman/printer — wrapper Go puro de las DLLs nativas
// (winspool.dll, kernel32.dll). Sin dependencias externas, sin drivers
// adicionales. La impresora que el cajero ya tiene instalada en Windows
// se ve acá directo.
package main

import (
	"fmt"

	"github.com/alexbrainman/printer"
)

// listPrinters devuelve los nombres de TODAS las impresoras instaladas
// en este Windows (locales + de red). El frontend muestra la lista y el
// admin elige cuál es de cocina y cuál de boletas.
func listPrinters() ([]string, error) {
	names, err := printer.ReadNames()
	if err != nil {
		return nil, fmt.Errorf("enumerar impresoras: %w", err)
	}
	return names, nil
}

// sendToPrinter manda bytes raw a la impresora elegida usando winspool.
// El payload viene ya con ESC/POS encoded (beep + corte) desde main.go.
// La impresora térmica los interpreta nativo.
func sendToPrinter(printerName string, data []byte) error {
	p, err := printer.Open(printerName)
	if err != nil {
		return fmt.Errorf("abrir impresora %q: %w", printerName, err)
	}
	defer p.Close()

	// "RAW" datatype = bytes crudos, sin que el driver intente "imprimir
	// como Word". Es lo que las térmicas ESC/POS necesitan.
	if err := p.StartRawDocument("FoodLabs Print"); err != nil {
		return fmt.Errorf("StartRawDocument: %w", err)
	}
	defer p.EndDocument()

	if err := p.StartPage(); err != nil {
		return fmt.Errorf("StartPage: %w", err)
	}
	defer p.EndPage()

	if _, err := p.Write(data); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}
