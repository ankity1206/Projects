// launcher/cmd/launcher/main.go
//
// PDFMaster Self-Bootstrapping Launcher
//
// This is the binary the user actually double-clicks.
// It runs BEFORE the main pdfmaster binary and is responsible for:
//   1. Detecting the environment (OS, arch, existing deps)
//   2. Extracting bundled libs/assets on first run
//   3. Showing a Bubble Tea setup UI if anything needs installing
//   4. Verifying integrity (sha256 checksums)
//   5. Setting LD_LIBRARY_PATH / PATH and exec()ing the real binary
//
// On subsequent launches it completes in < 50 ms (just a stat + exec).

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/yourname/pdfmaster/launcher/internal/selfextract"
	"github.com/yourname/pdfmaster/launcher/internal/setup"
	"github.com/yourname/pdfmaster/launcher/internal/syscheck"
	"github.com/yourname/pdfmaster/launcher/internal/ui"
)

const (
	AppName    = "PDFMaster"
	AppVersion = "1.0.0"
)

func main() {
	// Determine install dir:
	//   Linux:   ~/.local/share/pdfmaster/
	//   Windows: %LOCALAPPDATA%\PDFMaster\
	installDir, err := setup.InstallDir()
	if err != nil {
		fatal("Cannot determine install directory: %v", err)
	}

	// ── Fast path: already installed and healthy ─────────────────
	if setup.IsHealthy(installDir) {
		execMain(installDir)
		return // never reached
	}

	// ── First-run or repair path ─────────────────────────────────
	// Check what the system already has
	report := syscheck.Run()

	// Determine what needs to be done
	plan := setup.BuildPlan(installDir, report)

	if plan.NothingToDo() {
		// Mark healthy and launch
		_ = setup.WriteHealthFile(installDir)
		execMain(installDir)
		return
	}

	// Show the Bubble Tea setup UI
	if err := ui.RunSetupUI(plan, installDir); err != nil {
		// Non-interactive fallback: run headlessly
		fmt.Fprintf(os.Stderr, "[PDFMaster] Running setup (non-interactive)...\n")
		if err2 := setup.ExecutePlan(plan, installDir, nil); err2 != nil {
			fatal("Setup failed: %v", err2)
		}
	}

	// Verify the extracted payload
	if err := selfextract.VerifyChecksums(installDir); err != nil {
		fatal("Integrity check failed: %v", err)
	}

	// Write health marker so next launch is fast
	if err := setup.WriteHealthFile(installDir); err != nil {
		fmt.Fprintf(os.Stderr, "[warn] Could not write health file: %v\n", err)
	}

	execMain(installDir)
}

// execMain replaces the current process with the real pdfmaster binary.
func execMain(installDir string) {
	binName := "pdfmaster"
	if runtime.GOOS == "windows" {
		binName = "pdfmaster.exe"
	}
	binPath := filepath.Join(installDir, "bin", binName)

	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		fatal("Main binary not found at %s — try re-installing", binPath)
	}

	// Inject bundled lib directory into the loader path
	libDir := filepath.Join(installDir, "lib")
	injectLibPath(libDir)

	// Pass through all original arguments
	args := append([]string{binPath}, os.Args[1:]...)

	if runtime.GOOS == "windows" {
		// Windows: no syscall.Exec — use exec.Command + os.Exit
		cmd := exec.Command(binPath, os.Args[1:]...)
		cmd.Stdin  = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Linux/macOS: true exec (replaces process — zero overhead)
	if err := syscall.Exec(binPath, args, os.Environ()); err != nil {
		fatal("exec failed: %v", err)
	}
}

func injectLibPath(libDir string) {
	switch runtime.GOOS {
	case "linux", "darwin":
		existing := os.Getenv("LD_LIBRARY_PATH")
		if existing != "" {
			_ = os.Setenv("LD_LIBRARY_PATH", libDir+":"+existing)
		} else {
			_ = os.Setenv("LD_LIBRARY_PATH", libDir)
		}
	case "windows":
		existing := os.Getenv("PATH")
		_ = os.Setenv("PATH", libDir+";"+existing)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n[PDFMaster] FATAL: "+format+"\n", args...)
	os.Exit(1)
}
