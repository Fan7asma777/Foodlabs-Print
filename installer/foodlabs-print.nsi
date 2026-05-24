; FoodLabs Print Agent — NSIS Installer (M2)
;
; Instala el binary a C:\Program Files\FoodLabsPrintAgent\
; Crea entrada en el Inicio de Windows (HKCU\Run) → arranca solo cada vez
; que el cajero loguea en su sesión. Sin ventana negra (el binary tiene
; tray icon desde v0.2.0 M3).
;
; Desinstalable desde Panel de Control → Programas → FoodLabs Print Agent
;
; Build: makensis foodlabs-print.nsi (Windows runner con choco install nsis)

Unicode True
SetCompressor /SOLID lzma

!define APPNAME      "FoodLabs Print Agent"
!define COMPANY      "FoodLabs SpA"
!define VERSION      "0.3.4"
!define INSTALL_DIR  "$PROGRAMFILES64\FoodLabsPrintAgent"
!define UNINST_KEY   "Software\Microsoft\Windows\CurrentVersion\Uninstall\FoodLabsPrintAgent"
!define STARTUP_KEY  "Software\Microsoft\Windows\CurrentVersion\Run"

Name "${APPNAME}"
OutFile "FoodLabsPrintAgent-Setup.exe"
InstallDir "${INSTALL_DIR}"
RequestExecutionLevel admin

; UI: instalador tipo wizard simple. Páginas estándar de NSIS Modern UI 2.
!include "MUI2.nsh"

!define MUI_ABORTWARNING
; Icono del installer y uninstaller: matraz Foodlabs (foodlabs-logo.ico)
; generado en CI por ImageMagick desde assets/foodlabs-logo.png (multi-res:
; 16, 32, 48, 256). Reemplaza los íconos default del NSIS Modern UI.
!define MUI_ICON   "foodlabs-logo.ico"
!define MUI_UNICON "foodlabs-logo.ico"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_WELCOME
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "Spanish"

Section "Install"
  SetOutPath "$INSTDIR"

  ; Copia el binary. Asumimos que está en el mismo dir que el .nsi al compilar.
  File "FoodLabsPrintAgent.exe"

  ; Registro de desinstalación (Panel de Control)
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayName"     "${APPNAME}"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayVersion"  "${VERSION}"
  WriteRegStr HKLM "${UNINST_KEY}" "Publisher"       "${COMPANY}"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayIcon"     "$INSTDIR\FoodLabsPrintAgent.exe"
  WriteRegStr HKLM "${UNINST_KEY}" "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegStr HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoRepair" 1

  ; Auto-start: agregar al Run del usuario actual. Cuando logue, arranca solo.
  ; HKCU porque queremos que cada cajero arranque su propia instancia (no la
  ; del admin). Si querés "todos los users" cambiar a HKLM.
  WriteRegStr HKCU "${STARTUP_KEY}" "FoodLabsPrintAgent" '"$INSTDIR\FoodLabsPrintAgent.exe"'

  ; Crear uninstaller
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Arrancar el agent inmediatamente post-instalación. Sin esperar a próximo
  ; login — el cajero puede usar foodlabs ya mismo.
  Exec '"$INSTDIR\FoodLabsPrintAgent.exe"'
SectionEnd

Section "Uninstall"
  ; Matar proceso si está corriendo
  nsExec::ExecToLog 'taskkill /F /IM FoodLabsPrintAgent.exe'

  Delete "$INSTDIR\FoodLabsPrintAgent.exe"
  Delete "$INSTDIR\uninstall.exe"
  RMDir  "$INSTDIR"

  DeleteRegKey HKLM "${UNINST_KEY}"
  DeleteRegValue HKCU "${STARTUP_KEY}" "FoodLabsPrintAgent"
SectionEnd
