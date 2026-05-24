module github.com/foodlabs/print-agent

go 1.22

// `go mod tidy` resuelve versiones desde los imports. Sin pin de commits
// hardcodeados — más resiliente cuando las libs siguen siendo mantenidas.
// Imports relevantes:
//   - github.com/alexbrainman/printer (winspool API, windows-only build tag)
//   - github.com/getlantern/systray   (tray icon barra de tareas)
