module github.com/foodlabs/print-agent

go 1.22

// La require de `github.com/alexbrainman/printer` la resuelve `go mod tidy`
// automáticamente desde el import en printer_windows.go. Sin pin de versión
// acá, descarga la última master por defecto (más resiliente que un commit
// hash hardcodeado que podría volverse stale).
