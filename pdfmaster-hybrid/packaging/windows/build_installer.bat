@echo off
:: packaging/windows/build_installer.bat
::
:: Builds PDFMaster-Setup-1.0.0.exe and PDFMaster-Portable-1.0.0.zip
::
:: What this script does:
::   1. Locates and configures Visual Studio
::   2. Builds C++ engine (libpdfengine.lib) with CMake + MSVC
::   3. Builds pdfmaster.exe (Go + CGo)
::   4. Builds pdfmaster-launcher.exe (pure Go, no CGo)
::   5. Copies and bundles required DLLs (mupdf, qpdf, vcredist)
::   6. Runs makensis to produce the installer .exe
::   7. Creates a portable .zip (no installer needed)
::
:: Requirements (build machine):
::   - Visual Studio 2022 with "Desktop development with C++" workload
::   - Go 1.22+ in PATH
::   - CMake 3.22+ in PATH
::   - NSIS 3.x (for installer; portable zip works without it)
::   - MuPDF and QPDF either via vcpkg or manual MUPDF_ROOT/QPDF_ROOT
::
:: Usage:
::   build_installer.bat
::   build_installer.bat /portable-only
::   build_installer.bat /installer-only

setlocal EnableDelayedExpansion

set VERSION=1.0.0
set ROOT=%~dp0..\..
set BUILD_DIR=%ROOT%\build\windows-release
set DIST_DIR=%ROOT%\dist\windows
set OUTPUT_DIR=%ROOT%\dist

echo.
echo  ╔══════════════════════════════════════════════════╗
echo  ║    PDFMaster Windows Build  v%VERSION%             ║
echo  ╚══════════════════════════════════════════════════╝
echo.

:: ── Parse arguments ───────────────────────────────────────────────────
set DO_PORTABLE=1
set DO_INSTALLER=1
if "%1"=="/portable-only"  set DO_INSTALLER=0
if "%1"=="/installer-only" set DO_PORTABLE=0

:: ── Locate Visual Studio ──────────────────────────────────────────────
echo [1/7] Locating Visual Studio...

set VCVARS=
for %%Y in (2022 2019) do (
    for %%E in (Community Professional Enterprise) do (
        set CANDIDATE=
        if "%%Y"=="2022" set CANDIDATE=%ProgramFiles%\Microsoft Visual Studio\%%Y\%%E\VC\Auxiliary\Build\vcvars64.bat
        if "%%Y"=="2019" set CANDIDATE=%ProgramFiles(x86)%\Microsoft Visual Studio\%%Y\%%E\VC\Auxiliary\Build\vcvars64.bat
        if exist "!CANDIDATE!" (
            set VCVARS=!CANDIDATE!
            echo    Found VS %%Y %%E
            goto :vs_found
        )
    )
)
echo [WARN] Visual Studio not found — trying with current PATH
goto :vs_skip
:vs_found
call "%VCVARS%" >nul 2>&1
:vs_skip

:: ── Check Go ──────────────────────────────────────────────────────────
go version >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Go not found in PATH. Install from https://go.dev/dl/
    exit /b 1
)
echo    Go: 
go version

:: ── Set library paths (vcpkg or manual) ──────────────────────────────
echo.
echo [2/7] Configuring library paths...

if defined VCPKG_ROOT (
    set TOOLCHAIN=-DCMAKE_TOOLCHAIN_FILE="%VCPKG_ROOT%\scripts\buildsystems\vcpkg.cmake" -DVCPKG_TARGET_TRIPLET=x64-windows-static
    echo    Using vcpkg: %VCPKG_ROOT%
) else (
    set TOOLCHAIN=
    if defined MUPDF_ROOT  echo    MuPDF:  %MUPDF_ROOT%
    if defined QPDF_ROOT   echo    QPDF:   %QPDF_ROOT%
    if not defined MUPDF_ROOT if not defined QPDF_ROOT (
        echo [WARN] Neither VCPKG_ROOT nor MUPDF_ROOT/QPDF_ROOT set.
        echo [WARN] Set VCPKG_ROOT or set MUPDF_ROOT and QPDF_ROOT.
    )
)

:: ── Build C++ engine ──────────────────────────────────────────────────
echo.
echo [3/7] Building C++ engine (libpdfengine.lib)...

if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"

cmake -B "%BUILD_DIR%\engine" -S "%ROOT%\engine" ^
    -G "Visual Studio 17 2022" -A x64 ^
    -DCMAKE_BUILD_TYPE=Release ^
    -DCMAKE_POSITION_INDEPENDENT_CODE=ON ^
    -DENGINE_USE_SYSTEM_MUPDF=OFF ^
    -DENGINE_USE_SYSTEM_QPDF=OFF ^
    %TOOLCHAIN% ^
    %MUPDF_ROOT%:-DMUPDF_ROOT=%MUPDF_ROOT%% ^
    %QPDF_ROOT%:-DQPDF_ROOT=%QPDF_ROOT%% ^
    --log-level=WARNING

cmake --build "%BUILD_DIR%\engine" --config Release --parallel
echo    Engine built OK

:: ── Set CGo flags ─────────────────────────────────────────────────────
set CGO_ENABLED=1
set CGO_CFLAGS=-I%ROOT%\engine\include
set CGO_LDFLAGS=-L%BUILD_DIR%\engine\lib\Release -lpdfengine -lstdc++ -lm

:: ── Build main pdfmaster.exe ──────────────────────────────────────────
echo.
echo [4/7] Building pdfmaster.exe (Go + CGo)...

cd "%ROOT%\go"
go build ^
    -ldflags="-s -w -X main.buildVersion=%VERSION%" ^
    -o "%BUILD_DIR%\pdfmaster.exe" ^
    .\cmd\pdfmaster

echo    pdfmaster.exe built OK

:: ── Build launcher ─────────────────────────────────────────────────────
echo.
echo [5/7] Building pdfmaster-launcher.exe (pure Go)...

cd "%ROOT%\launcher"
set CGO_ENABLED=0
go build ^
    -ldflags="-s -w -H windowsgui -X main.AppVersion=%VERSION%" ^
    -o "%BUILD_DIR%\pdfmaster-launcher.exe" ^
    .\cmd\launcher

echo    pdfmaster-launcher.exe built OK
:: Restore CGo
set CGO_ENABLED=1

:: ── Collect DLLs ──────────────────────────────────────────────────────
echo.
echo [6/7] Collecting DLLs...

mkdir "%DIST_DIR%\lib" 2>nul
mkdir "%DIST_DIR%\bin" 2>nul

copy "%BUILD_DIR%\pdfmaster-launcher.exe" "%DIST_DIR%\pdfmaster-launcher.exe" >nul
copy "%BUILD_DIR%\pdfmaster.exe"          "%DIST_DIR%\pdfmaster.exe"          >nul
copy "%ROOT%\docs\USER_MANUAL.md"         "%DIST_DIR%\USER_MANUAL.md"         >nul 2>nul
copy "%ROOT%\README.md"                   "%DIST_DIR%\README.md"              >nul 2>nul
copy "%ROOT%\LICENSE"                     "%DIST_DIR%\LICENSE"                >nul 2>nul

:: Copy MuPDF and QPDF DLLs from vcpkg or manual paths
if defined VCPKG_ROOT (
    set VCPKG_BIN=%VCPKG_ROOT%\installed\x64-windows\bin
    for %%f in (mupdf.dll qpdf29.dll zlib1.dll libpng16.dll libjpeg-turbo8.dll) do (
        if exist "%VCPKG_BIN%\%%f" (
            copy "%VCPKG_BIN%\%%f" "%DIST_DIR%\" >nul
            echo    Bundled: %%f
        )
    )
)

:: Run windeployqt if Qt is being bundled
where windeployqt >nul 2>&1
if not errorlevel 1 (
    echo    Running windeployqt...
    windeployqt --release --no-translations --no-system-d3d-compiler ^
        "%DIST_DIR%\pdfmaster-launcher.exe" 2>nul
)

:: ── Create portable ZIP ────────────────────────────────────────────────
if "%DO_PORTABLE%"=="1" (
    echo.
    echo [7a/7] Creating portable ZIP...
    mkdir "%OUTPUT_DIR%" 2>nul
    set ZIP_FILE=%OUTPUT_DIR%\PDFMaster-Portable-%VERSION%.zip

    :: Use PowerShell to create zip (available on Win10+)
    powershell -NoProfile -Command ^
        "Compress-Archive -Path '%DIST_DIR%\*' -DestinationPath '%ZIP_FILE%' -Force"

    echo    Portable ZIP: %ZIP_FILE%
)

:: ── Run NSIS installer ────────────────────────────────────────────────
if "%DO_INSTALLER%"=="1" (
    echo.
    echo [7b/7] Building NSIS installer...

    where makensis >nul 2>&1
    if errorlevel 1 (
        echo [WARN] makensis not found — skipping installer.
        echo [WARN] Install NSIS from https://nsis.sourceforge.io/
        goto :skip_nsis
    )

    cd "%ROOT%\packaging\windows\nsis"
    makensis /V3 pdfmaster.nsi
    echo    Installer: %OUTPUT_DIR%\PDFMaster-Setup-%VERSION%.exe
    :skip_nsis
)

:: ── Done ──────────────────────────────────────────────────────────────
echo.
echo  ╔══════════════════════════════════════════════════╗
echo  ║  Build complete!                                  ║
echo  ╚══════════════════════════════════════════════════╝
echo.
if "%DO_INSTALLER%"=="1" echo   Installer : %OUTPUT_DIR%\PDFMaster-Setup-%VERSION%.exe
if "%DO_PORTABLE%"=="1"  echo   Portable  : %OUTPUT_DIR%\PDFMaster-Portable-%VERSION%.zip
echo.
echo   Launcher  : %DIST_DIR%\pdfmaster-launcher.exe
echo   Binary    : %DIST_DIR%\pdfmaster.exe
echo.
pause
