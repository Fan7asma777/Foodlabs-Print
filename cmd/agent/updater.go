// Auto-update (M4). Chequea GitHub Releases API del repo público cada 6h,
// si hay versión nueva descarga el .exe y se reemplaza solo.
//
// Library: github.com/minio/selfupdate (Apache 2.0). Maneja el reemplazo
// del binary "vivo" en Windows usando MoveFileEx (que renombra el viejo y
// pone el nuevo en su lugar). Después llamamos a restartSelf() para que
// el proceso nuevo arranque y el viejo termine.
//
// Si auto-update falla (red caída, GitHub rate-limit, hash mismatch),
// loggea warn y sigue corriendo con la versión actual — NO debe romper
// operación del cajero.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/minio/selfupdate"
)

const (
	updateCheckInterval = 6 * time.Hour
	releasesAPIURL      = "https://api.github.com/repos/Fan7asma777/Foodlabs-Print/releases/latest"
)

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// startAutoUpdater arranca el loop de chequeo en background. Bloquea hasta
// el ctx done. Pensado para correr en goroutine desde main.
func startAutoUpdater() {
	go func() {
		// Primera chequeada al minuto de arrancar (no inmediata — dejamos
		// que el agent se asiente primero).
		time.Sleep(1 * time.Minute)
		checkAndUpdate()
		t := time.NewTicker(updateCheckInterval)
		defer t.Stop()
		for range t.C {
			checkAndUpdate()
		}
	}()
}

func checkAndUpdate() {
	latest, err := fetchLatestRelease()
	if err != nil {
		log.Printf("auto-update: chequeo falló: %v", err)
		return
	}
	if !isNewer(latest.TagName, version) {
		log.Printf("auto-update: ya estás en %s (última %s)", version, latest.TagName)
		return
	}

	asset := findBinaryAsset(latest)
	if asset == nil {
		log.Printf("auto-update: no encontré asset FoodLabsPrintAgent.exe en %s", latest.TagName)
		return
	}

	log.Printf("auto-update: descargando %s (%d bytes)…", latest.TagName, asset.Size)
	if err := applyUpdate(asset.BrowserDownloadURL); err != nil {
		log.Printf("auto-update: aplicar falló: %v", err)
		return
	}
	log.Printf("auto-update: instalado %s → reiniciando", latest.TagName)
	restartSelf()
}

func fetchLatestRelease() (*ghRelease, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("GET", releasesAPIURL, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "FoodLabsPrintAgent/"+version)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("github releases API HTTP %d", res.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(res.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// findBinaryAsset busca el asset que es el binary portable (no el installer
// Setup.exe). Auto-update reemplaza el binary EN VIVO; el installer se
// usa solo para primera instalación.
func findBinaryAsset(rel *ghRelease) *struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
} {
	for i := range rel.Assets {
		a := &rel.Assets[i]
		// Nombre exacto del binary portable. Excluimos Setup.exe del installer.
		if a.Name == "FoodLabsPrintAgent.exe" {
			return a
		}
	}
	return nil
}

// isNewer compara strings de versión tipo "print-agent-v0.2.0" vs "0.1.0".
// Parsing simple — asume semver major.minor.patch.
func isNewer(tag, current string) bool {
	tagVer := strings.TrimPrefix(tag, "print-agent-v")
	tagVer = strings.TrimPrefix(tagVer, "v")
	return semverGreater(tagVer, current)
}

func semverGreater(a, b string) bool {
	parse := func(s string) [3]int {
		var v [3]int
		parts := strings.Split(s, ".")
		for i := 0; i < 3 && i < len(parts); i++ {
			var n int
			_, _ = fmt.Sscanf(parts[i], "%d", &n)
			v[i] = n
		}
		return v
	}
	av, bv := parse(a), parse(b)
	for i := 0; i < 3; i++ {
		if av[i] != bv[i] {
			return av[i] > bv[i]
		}
	}
	return false
}

func applyUpdate(url string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	res, err := client.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("download HTTP %d", res.StatusCode)
	}
	// selfupdate.Apply reemplaza el binary EN VIVO de forma segura
	// (renombra viejo, escribe nuevo, en Windows usa MoveFileEx con
	// MOVEFILE_DELAY_UNTIL_REBOOT como fallback).
	return selfupdate.Apply(res.Body, selfupdate.Options{})
}

// restartSelf arranca una nueva instancia del binary y termina la actual.
// El nuevo proceso es independiente — el sistema cierra el viejo cuando
// retornamos de main.
func restartSelf() {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("restartSelf: no pude resolver exe: %v", err)
		return
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// /B = no consola visible (el binary nuevo tiene tray icon)
		cmd = exec.Command("cmd", "/C", "start", "/B", "", exe)
	} else {
		cmd = exec.Command(exe)
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		log.Printf("restartSelf: start falló: %v", err)
		return
	}
	log.Printf("restartSelf: nuevo proceso iniciado, terminando viejo en 2s…")
	time.Sleep(2 * time.Second)
	os.Exit(0)
}
