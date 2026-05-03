; ═══════════════════════════════════════════════════════════════════════════════
; Skreen Agent Installer
; NSIS 3.x + Modern UI 2
; ═══════════════════════════════════════════════════════════════════════════════

!include "MUI2.nsh"
!include "FileFunc.nsh"

; ── Application Metadata ──────────────────────────────────────────────────────
!define APP_NAME        "Skreen Agent"
!define APP_VERSION     "1.0.0"
!define APP_PUBLISHER   "Skreen"
!define APP_URL         "https://skreen.io"
!define APP_EXE         "skreen-agent.exe"
!define TASK_NAME       "Skreen_Agent"
!define REG_UNINSTALL   "Software\Microsoft\Windows\CurrentVersion\Uninstall\SkreenAgent"

; ── Installer Output ──────────────────────────────────────────────────────────
Name            "${APP_NAME} ${APP_VERSION}"
OutFile         "..\installer\skreen-agent-setup.exe"
InstallDir      "$PROGRAMFILES64\Skreen"
InstallDirRegKey HKLM "${REG_UNINSTALL}" "InstallLocation"
RequestExecutionLevel admin
ShowInstDetails  hide
ShowUnInstDetails hide
SetCompressor    /SOLID lzma

; ── MUI Appearance ────────────────────────────────────────────────────────────
!define MUI_ABORTWARNING

!define MUI_WELCOMEPAGE_TITLE   "Welcome to Skreen Remote Access"
!define MUI_WELCOMEPAGE_TEXT    "This wizard will install the Skreen Agent on your \
computer.$\n$\nYour technician will be able to view and control your screen \
securely once installation is complete.$\n$\nClick Next to continue."

!define MUI_LICENSEPAGE_RADIOBUTTONS

!define MUI_DIRECTORYPAGE_TEXT_TOP \
    "Skreen Agent will be installed in the folder below.$\n\
    To install in a different folder, click Browse and select another folder."

!define MUI_INSTFILESPAGE_PROGRESSBAR "smooth"

!define MUI_FINISHPAGE_TITLE    "Installation Complete"
!define MUI_FINISHPAGE_TEXT     "Skreen Agent has been installed and is now running \
in the background.$\n$\nYour technician can now connect to this computer. You may \
safely close this window and delete the installer file."
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_FINISHPAGE_SHOWREADME ""
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED

; ── Pages ─────────────────────────────────────────────────────────────────────
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE     "assets\license.txt"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; ── Language ──────────────────────────────────────────────────────────────────
!insertmacro MUI_LANGUAGE "English"

; ══════════════════════════════════════════════════════════════════════════════
; INSTALL SECTION
; ══════════════════════════════════════════════════════════════════════════════
Section "Skreen Agent" SecMain
    SectionIn RO    ; Required — user cannot deselect

    DetailPrint "Preparing installation directory..."
    SetOutPath "$INSTDIR"
    SetOverwrite on

    DetailPrint "Copying Skreen Agent..."
    File "..\agent\skreen-agent.exe"

    DetailPrint "Registering uninstaller..."
    WriteUninstaller "$INSTDIR\uninstall.exe"

    ; ── Add/Remove Programs Entry ────────────────────────────────────────────
    WriteRegStr   HKLM "${REG_UNINSTALL}" "DisplayName"     "${APP_NAME}"
    WriteRegStr   HKLM "${REG_UNINSTALL}" "DisplayVersion"  "${APP_VERSION}"
    WriteRegStr   HKLM "${REG_UNINSTALL}" "Publisher"       "${APP_PUBLISHER}"
    WriteRegStr   HKLM "${REG_UNINSTALL}" "URLInfoAbout"    "${APP_URL}"
    WriteRegStr   HKLM "${REG_UNINSTALL}" "InstallLocation" "$INSTDIR"
    WriteRegStr   HKLM "${REG_UNINSTALL}" "UninstallString" '"$INSTDIR\uninstall.exe"'
    WriteRegStr   HKLM "${REG_UNINSTALL}" "DisplayIcon"     "$INSTDIR\${APP_EXE}"
    WriteRegDWORD HKLM "${REG_UNINSTALL}" "NoModify"        1
    WriteRegDWORD HKLM "${REG_UNINSTALL}" "NoRepair"        1

    ; Estimate size for Control Panel display
    ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
    IntFmt $0 "0x%08X" $0
    WriteRegDWORD HKLM "${REG_UNINSTALL}" "EstimatedSize" "$0"

    ; ── Start Menu Shortcuts ─────────────────────────────────────────────────
    DetailPrint "Creating Start Menu shortcuts..."
    CreateDirectory "$SMPROGRAMS\Skreen"
    CreateShortcut "$SMPROGRAMS\Skreen\Skreen Agent.lnk" \
        "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_EXE}" 0
    CreateShortcut "$SMPROGRAMS\Skreen\Uninstall Skreen Agent.lnk" \
        "$INSTDIR\uninstall.exe"

    ; ── Persistence: Scheduled Task (survives reboots) ───────────────────────
    DetailPrint "Registering system service..."
    ; Remove old task silently if it exists
    ExecWait 'schtasks /delete /tn "${TASK_NAME}" /f'
    ; Create task: runs at logon for any user, highest privilege
    ExecWait 'schtasks /create /tn "${TASK_NAME}" /tr "$\"$INSTDIR\${APP_EXE}$\" -installer $\"$EXEFILE$\"" /sc onlogon /ru "" /rl HIGHEST /f'

    ; ── Start Agent Now ───────────────────────────────────────────────────────
    DetailPrint "Starting Skreen Agent..."
    Exec '"$INSTDIR\${APP_EXE}" -installer "$EXEFILE"'

    DetailPrint "Installation complete."
SectionEnd

; ══════════════════════════════════════════════════════════════════════════════
; UNINSTALL SECTION
; ══════════════════════════════════════════════════════════════════════════════
Section "Uninstall"
    ; Stop running agent
    ExecWait 'taskkill /f /im "${APP_EXE}"'

    ; Remove scheduled task
    ExecWait 'schtasks /end /tn "${TASK_NAME}"'
    ExecWait 'schtasks /delete /tn "${TASK_NAME}" /f'

    ; Remove files
    Delete "$INSTDIR\${APP_EXE}"
    Delete "$INSTDIR\uninstall.exe"
    RMDir  "$INSTDIR"

    ; Remove Start Menu
    Delete "$SMPROGRAMS\Skreen\Skreen Agent.lnk"
    Delete "$SMPROGRAMS\Skreen\Uninstall Skreen Agent.lnk"
    RMDir  "$SMPROGRAMS\Skreen"

    ; Remove registry entries
    DeleteRegKey HKLM "${REG_UNINSTALL}"
SectionEnd
