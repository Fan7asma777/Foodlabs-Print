//go:build !windows

// Stub para Linux/Mac. M1 = solo Windows; estos OSes vienen en M4.
// Si alguien intenta correr el binario en Mac/Linux le devuelve error
// claro en lugar de crashear.
package main

import "errors"

func listPrinters() ([]string, error) {
	return nil, errors.New("FoodLabs Print Agent M1 solo soporta Windows — Mac/Linux planeado en M4")
}

func sendToPrinter(printerName string, data []byte) error {
	return errors.New("FoodLabs Print Agent M1 solo soporta Windows — Mac/Linux planeado en M4")
}
