# FoodLabs Print Agent

> Mini-server local en la PC del cajero que hace de **puente** entre `foodlabs.app` (HTTPS, en la nube) y la impresora térmica conectada por **USB, Bluetooth o LAN**.

## ¿Por qué este sub-proyecto existe?

Los navegadores web (Chrome/Edge/Safari) **bloquean** por seguridad que una página HTTPS hable directo con dispositivos USB o impresoras de red local. La única excepción permitida es llamar a `http://127.0.0.1:<puerto>` desde una página HTTPS — eso es lo que aprovecha este agent.

Hoy usamos **Parzibyte Webapp** (third-party gratuito) como agent. Estable, pero:
- Marca de un tercero (el cliente desconfía)
- Sin telemetría — no sabemos si está corriendo desde el servidor
- Sin auto-update — los parches requieren reinstalación manual
- Sin branding Foodlabs

**Este sub-proyecto reemplaza Parzibyte con un agent propio.**

---

## Stack decidido

| Componente | Tecnología | Por qué |
|---|---|---|
| Lenguaje | **Go** | Binario único pequeño (~10MB), cross-compile trivial, libs ESC/POS maduras, sin runtime dependency en la PC del cliente |
| ESC/POS | [`mugli/escpos`](https://github.com/mugli/escpos) o [`hennedo/escpos`](https://github.com/hennedo/escpos) | Probadas en producción, soportan tickets, QR, códigos de barras |
| USB raw | [`google/gousb`](https://github.com/google/gousb) (libusb wrapper) | Acceso a impresoras USB sin drivers Windows propietarios |
| HTTP server | `net/http` stdlib | Sin frameworks externos — minimiza superficie |
| Tray icon | [`getlantern/systray`](https://github.com/getlantern/systray) | Visual cue al cajero ("running", "printer ok") |
| Empaquetado | `goreleaser` + WiX (MSI) o NSIS (.exe installer) | Standard de la industria |
| Service Windows | Tarea programada al arranque (más simple que NSSM) | El cajero abre el local, el agent arranca solo |
| Auto-update | [`minio/selfupdate`](https://github.com/minio/selfupdate) o similar | Chequea release nuevo al inicio y se reemplaza solo |

---

## API HTTP del agent

**Drop-in compatible con Parzibyte** para que el frontend de Foodlabs NO requiera cambios.

| Endpoint | Método | Descripción |
|---|---|---|
| `GET /health` | GET | `{ok: true, version: "x.y.z"}` |
| `GET /impresoras` | GET | Lista impresoras instaladas (compatible Parzibyte) |
| `POST /imprimir` | POST | Recibe ticket ESC/POS / texto plano y lo imprime |
| `GET /version` | GET | Versión del agent (para auto-update) |

Puerto: **`40213`** (mismo que Parzibyte para drop-in). Bind solo a `127.0.0.1` — **NO accesible desde la red**.

---

## Milestones

### M1 — MVP Windows (~3-5 días)
- [ ] `cmd/agent/main.go`: HTTP server stub en `127.0.0.1:40213`
- [ ] `GET /impresoras`: lista impresoras Windows usando `winspool` API
- [ ] `POST /imprimir`: recibe payload texto + lo imprime en la default printer
- [ ] Build con `go build` → `print-agent.exe`
- [ ] Smoke test con impresora térmica USB real

### M2 — Distribución (~3-4 días)
- [ ] Empaquetado con NSIS installer (`.exe` que copia binary + crea tarea programada al arranque)
- [ ] Code signing del binary (necesita certificado OV ~$200/año)
- [ ] GitHub Release con el `.exe` firmado
- [ ] Página en `foodlabs.app/print-agent` con botón de descarga
- [ ] Frontend: cambiar `PrinterSetupModal` a apuntar al binary nuestro

### M3 — UX & Soporte (~3-4 días)
- [ ] Tray icon con estados (verde/amber/rojo)
- [ ] Logs rotativos en `%APPDATA%/FoodLabsPrintAgent/logs/`
- [ ] Telemetría opt-in (versión + status enviado a CEREBRO cada 1h)
- [ ] Auto-update al arranque

### M4 — Multi-OS (luego, si justifica)
- [ ] Build Mac (Apple Notarization ~$99/año)
- [ ] Build Linux (.deb/.rpm)

---

## Decisiones explícitas

1. **NO usamos Electron** — agregaría ~150MB de Chromium innecesario para un agent sin UI.
2. **NO usamos Node+pkg** — binarios de 50MB y dependencia de runtime escondida; Go es mejor para este caso.
3. **NO escuchamos en `0.0.0.0`** — solo loopback, evita que un atacante en la red local del local imprima ad-hoc.
4. **NO accedemos a internet desde el agent** salvo para auto-update (signed releases) y telemetría opt-in.
5. **API drop-in compatible Parzibyte** para que la transición de Parzibyte → FoodLabs Print Agent sea cero cambios en frontend.

---

## Cómo se construye y se libera

**No hace falta tener Go instalado local.** GitHub Actions builda automático:

| Trigger | Qué pasa |
|---|---|
| Push a `main` que toca `lab/print-agent/**` | Build + smoke test + artifact (descargable desde la pestaña Actions, 30 días) |
| Push de un tag `print-agent-v*` (ej. `print-agent-v0.1.0`) | Build + crea GitHub Release con el `.exe` como asset descargable público |
| Manual desde la UI de Actions | Build a demanda |

**Workflow:** `.github/workflows/print-agent.yml`

**Para liberar una versión nueva al cliente:**
```bash
git tag print-agent-v0.1.0
git push origin print-agent-v0.1.0
# GitHub Actions construye + crea Release
# URL del .exe queda fija:
# https://github.com/Fan7asma777/Foodlabs/releases/download/print-agent-v0.1.0/FoodLabsPrintAgent.exe
```

La UI de Foodlabs apunta a esa URL.

---

## Cómo desarrollar local (opcional — si tenés Go instalado)

```bash
cd lab/print-agent
go mod tidy           # descarga deps
go build -o FoodLabsPrintAgent.exe ./cmd/agent
./FoodLabsPrintAgent.exe

# Output esperado:
# [FoodLabs Print Agent] v0.1.0 — escuchando en http://127.0.0.1:40213
```

Test manual con curl:
```bash
# Listar impresoras
curl http://127.0.0.1:40213/impresoras

# Imprimir un ticket de prueba
curl -X POST http://127.0.0.1:40213/imprimir \
  -H "Content-Type: application/json" \
  -d '{"impresora":"TM-T20","texto":"=== FOODLABS TEST ===\nTicket de prueba\n","cut":true,"beep":true}'
```

---

## Estado actual

**v0.1.0 (M1 MVP) — listo para tag + release.** Código completo, workflow CI configurado.

Tasks pendientes M1 → completar v0.1:
- [x] HTTP server stub
- [x] Listar impresoras Windows (winspool)
- [x] Imprimir texto raw con ESC/POS
- [x] Corte automático del papel
- [x] Beep cocina
- [x] GitHub Actions build & release
- [ ] Tag `print-agent-v0.1.0` → primera release pública
- [ ] Test en hardware real (impresora térmica USB)
- [ ] Actualizar UI Foodlabs para apuntar al release

Coordinación: `memory/session_state.md` marca este sub-proyecto como WIP de `claude-qa-kike`.
