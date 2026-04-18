; packaging/windows/nsis/pdfmaster.nsi
;
; PDFMaster Windows Installer — NSIS script
;
; Creates:
;   - Silent install to %LOCALAPPDATA%\PDFMaster\
;   - Start Menu shortcut
;   - Desktop shortcut (optional)
;   - "Open with PDFMaster" right-click context menu for .pdf files
;   - Full uninstaller
;   - No admin/UAC required (per-user install)
;
; Build: makensis pdfmaster.nsi
; Requires NSIS 3.x: https://nsis.sourceforge.io/

Unicode True

;--------------------------------
; General
;--------------------------------
!define APP_NAME        "PDFMaster"
!define APP_VERSION     "1.0.0"
!define APP_PUBLISHER   "PDFMaster Project"
!define APP_URL         "https://github.com/yourname/pdfmaster"
!define APP_EXE         "pdfmaster-launcher.exe"
!define MAIN_EXE        "pdfmaster.exe"
!define UNINSTALL_KEY   "Software\Microsoft\Windows\CurrentVersion\Uninstall\PDFMaster"
!define REGISTRY_KEY    "Software\PDFMaster"

Name          "${APP_NAME} ${APP_VERSION}"
OutFile       "..\..\..\dist\PDFMaster-Setup-${APP_VERSION}.exe"
InstallDir    "$LOCALAPPDATA\${APP_NAME}"
InstallDirRegKey HKCU "${REGISTRY_KEY}" "InstallDir"
RequestExecutionLevel user   ; NO UAC — user-level install only
ShowInstDetails show
ShowUnInstDetails show

SetCompressor /SOLID lzma
SetCompressorDictSize 64

;--------------------------------
; Modern UI
;--------------------------------
!include "MUI2.nsh"
!include "FileFunc.nsh"

!define MUI_ABORTWARNING
!define MUI_ICON        "..\..\..\engine\resources\icons\pdfmaster.ico"
!define MUI_UNICON      "..\..\..\engine\resources\icons\pdfmaster.ico"

; Welcome page
!define MUI_WELCOMEPAGE_TITLE    "Welcome to PDFMaster ${APP_VERSION}"
!define MUI_WELCOMEPAGE_TEXT     "This installer will set up PDFMaster on your computer.$\r$\n$\r$\nPDFMaster is a high-performance, offline PDF tool.$\r$\n$\r$\nNo internet connection required.$\r$\n$\r$\nClick Next to continue."
!define MUI_WELCOMEFINISHPAGE_BITMAP "welcome_banner.bmp"

; Finish page
!define MUI_FINISHPAGE_TITLE     "Installation Complete"
!define MUI_FINISHPAGE_TEXT      "PDFMaster has been installed successfully.$\r$\n$\r$\nClick Finish to close the installer."
!define MUI_FINISHPAGE_RUN       "$INSTDIR\${APP_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT  "Launch PDFMaster"
!define MUI_FINISHPAGE_LINK      "Visit PDFMaster on GitHub"
!define MUI_FINISHPAGE_LINK_LOCATION "${APP_URL}"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE    "..\..\..\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

;--------------------------------
; Version information
;--------------------------------
VIProductVersion "${APP_VERSION}.0"
VIAddVersionKey /LANG=${LANG_ENGLISH} "ProductName"      "${APP_NAME}"
VIAddVersionKey /LANG=${LANG_ENGLISH} "ProductVersion"   "${APP_VERSION}"
VIAddVersionKey /LANG=${LANG_ENGLISH} "CompanyName"      "${APP_PUBLISHER}"
VIAddVersionKey /LANG=${LANG_ENGLISH} "FileDescription"  "${APP_NAME} Installer"
VIAddVersionKey /LANG=${LANG_ENGLISH} "FileVersion"      "${APP_VERSION}.0"
VIAddVersionKey /LANG=${LANG_ENGLISH} "LegalCopyright"   "Open Source"

;--------------------------------
; Installation
;--------------------------------
Section "PDFMaster (required)" SecMain
    SectionIn RO  ; cannot be deselected

    SetOutPath "$INSTDIR"

    ; ── Core binaries ───────────────────────────────────────────
    File "..\..\..\dist\windows\pdfmaster-launcher.exe"
    File "..\..\..\dist\windows\pdfmaster.exe"

    ; ── Bundled DLLs ────────────────────────────────────────────
    File /r "..\..\..\dist\windows\lib\*.dll"

    ; ── Documentation ───────────────────────────────────────────
    File "..\..\..\docs\USER_MANUAL.md"
    File "..\..\..\README.md"
    File "..\..\..\LICENSE"

    ; ── Write registry ───────────────────────────────────────────
    WriteRegStr   HKCU "${REGISTRY_KEY}" "InstallDir"  "$INSTDIR"
    WriteRegStr   HKCU "${REGISTRY_KEY}" "Version"     "${APP_VERSION}"
    WriteRegStr   HKCU "${REGISTRY_KEY}" "Executable"  "$INSTDIR\${APP_EXE}"

    ; ── Uninstaller ──────────────────────────────────────────────
    WriteUninstaller "$INSTDIR\Uninstall.exe"

    ; Add/Remove Programs entry (no admin needed via HKCU)
    WriteRegStr   HKCU "${UNINSTALL_KEY}" "DisplayName"          "${APP_NAME}"
    WriteRegStr   HKCU "${UNINSTALL_KEY}" "DisplayVersion"       "${APP_VERSION}"
    WriteRegStr   HKCU "${UNINSTALL_KEY}" "Publisher"            "${APP_PUBLISHER}"
    WriteRegStr   HKCU "${UNINSTALL_KEY}" "URLInfoAbout"         "${APP_URL}"
    WriteRegStr   HKCU "${UNINSTALL_KEY}" "DisplayIcon"          "$INSTDIR\${APP_EXE}"
    WriteRegStr   HKCU "${UNINSTALL_KEY}" "UninstallString"      "$INSTDIR\Uninstall.exe"
    WriteRegStr   HKCU "${UNINSTALL_KEY}" "QuietUninstallString" "$INSTDIR\Uninstall.exe /S"
    WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify"             1
    WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair"             1

    ; Estimated size
    ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
    IntFmt $0 "0x%08X" $0
    WriteRegDWORD HKCU "${UNINSTALL_KEY}" "EstimatedSize" "$0"

SectionEnd

;--------------------------------
; Optional: Start Menu shortcut
;--------------------------------
Section "Start Menu Shortcut" SecStartMenu
    CreateDirectory "$SMPROGRAMS\${APP_NAME}"
    CreateShortcut  "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk" \
                    "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_EXE}" 0
    CreateShortcut  "$SMPROGRAMS\${APP_NAME}\Uninstall.lnk" \
                    "$INSTDIR\Uninstall.exe"
SectionEnd

;--------------------------------
; Optional: Desktop shortcut
;--------------------------------
Section /o "Desktop Shortcut" SecDesktop
    CreateShortcut "$DESKTOP\${APP_NAME}.lnk" \
                   "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_EXE}" 0
SectionEnd

;--------------------------------
; Optional: PDF file association
;--------------------------------
Section "Open PDF files with PDFMaster" SecAssoc
    ; Register as a handler for .pdf (user-level, no admin)
    WriteRegStr HKCU "Software\Classes\.pdf\OpenWithProgids" "PDFMaster.Document" ""
    WriteRegStr HKCU "Software\Classes\PDFMaster.Document" "" "PDF Document"
    WriteRegStr HKCU "Software\Classes\PDFMaster.Document\DefaultIcon" "" \
                     "$INSTDIR\${APP_EXE},0"
    WriteRegStr HKCU "Software\Classes\PDFMaster.Document\shell\open\command" "" \
                     '"$INSTDIR\${APP_EXE}" "%1"'
    ; Context menu "Open with PDFMaster"
    WriteRegStr HKCU "Software\Classes\SystemFileAssociations\.pdf\shell\PDFMaster" \
                     "" "Open with PDFMaster"
    WriteRegStr HKCU "Software\Classes\SystemFileAssociations\.pdf\shell\PDFMaster\command" \
                     "" '"$INSTDIR\${APP_EXE}" "%1"'
    ; Notify shell
    System::Call 'Shell32::SHChangeNotify(i 0x8000000, i 0, i 0, i 0)'
SectionEnd

;--------------------------------
; Section descriptions (tooltip)
;--------------------------------
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
    !insertmacro MUI_DESCRIPTION_TEXT ${SecMain}      "Core PDFMaster application (required)"
    !insertmacro MUI_DESCRIPTION_TEXT ${SecStartMenu} "Add PDFMaster to the Start Menu"
    !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop}   "Add a shortcut to the Desktop"
    !insertmacro MUI_DESCRIPTION_TEXT ${SecAssoc}     "Right-click any PDF to open with PDFMaster"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

;--------------------------------
; Uninstall
;--------------------------------
Section "Uninstall"
    ; Remove files
    Delete "$INSTDIR\pdfmaster-launcher.exe"
    Delete "$INSTDIR\pdfmaster.exe"
    Delete "$INSTDIR\*.dll"
    Delete "$INSTDIR\USER_MANUAL.md"
    Delete "$INSTDIR\README.md"
    Delete "$INSTDIR\LICENSE"
    Delete "$INSTDIR\Uninstall.exe"
    RMDir  "$INSTDIR"

    ; Remove shortcuts
    Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk"
    Delete "$SMPROGRAMS\${APP_NAME}\Uninstall.lnk"
    RMDir  "$SMPROGRAMS\${APP_NAME}"
    Delete "$DESKTOP\${APP_NAME}.lnk"

    ; Remove registry
    DeleteRegKey HKCU "${UNINSTALL_KEY}"
    DeleteRegKey HKCU "${REGISTRY_KEY}"
    DeleteRegKey HKCU "Software\Classes\PDFMaster.Document"
    DeleteRegValue HKCU "Software\Classes\.pdf\OpenWithProgids" "PDFMaster.Document"
    DeleteRegKey HKCU "Software\Classes\SystemFileAssociations\.pdf\shell\PDFMaster"

    System::Call 'Shell32::SHChangeNotify(i 0x8000000, i 0, i 0, i 0)'
SectionEnd
