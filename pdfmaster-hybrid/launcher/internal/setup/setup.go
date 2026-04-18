// launcher/internal/setup/setup.go
//
// Determines what needs to happen (the Plan) and executes it.
// Never calls apt/pacman/winget — always uses bundled assets.

package setup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/yourname/pdfmaster/launcher/internal/syscheck"
)

// ── Install directory ─────────────────────────────────────────────────

// InstallDir returns the per-user install directory.
//   Linux:   ~/.local/share/pdfmaster/
//   Windows: %LOCALAPPDATA%\PDFMaster\
func InstallDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(base, "PDFMaster"), nil
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "pdfmaster"), nil
	}
}

// ── Health file ───────────────────────────────────────────────────────

type healthFile struct {
	Version    string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
	Checksum   string    `json:"checksum"` // of pdfmaster binary
}

const healthFileName = ".health.json"
const appVersion     = "1.0.0"

// IsHealthy returns true if the install dir has a valid health file
// AND the main binary exists.
func IsHealthy(installDir string) bool {
	hpath := filepath.Join(installDir, healthFileName)
	data, err := os.ReadFile(hpath)
	if err != nil {
		return false
	}
	var h healthFile
	if err := json.Unmarshal(data, &h); err != nil {
		return false
	}
	if h.Version != appVersion {
		return false // version bump → re-extract
	}
	// Check binary exists
	binName := "pdfmaster"
	if runtime.GOOS == "windows" {
		binName = "pdfmaster.exe"
	}
	binPath := filepath.Join(installDir, "bin", binName)
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return false
	}
	return true
}

// WriteHealthFile writes the health marker after successful install.
func WriteHealthFile(installDir string) error {
	binName := "pdfmaster"
	if runtime.GOOS == "windows" {
		binName = "pdfmaster.exe"
	}
	binPath := filepath.Join(installDir, "bin", binName)

	cksum := ""
	if f, err := os.Open(binPath); err == nil {
		h := sha256.New()
		_, _ = io.Copy(h, f)
		f.Close()
		cksum = hex.EncodeToString(h.Sum(nil))
	}

	h := healthFile{
		Version:     appVersion,
		InstalledAt: time.Now().UTC(),
		Checksum:    cksum,
	}
	data, _ := json.MarshalIndent(h, "", "  ")
	return os.WriteFile(filepath.Join(installDir, healthFileName), data, 0o644)
}

// ── Plan ──────────────────────────────────────────────────────────────

// Step represents a single setup action.
type Step struct {
	ID          string // unique identifier
	Title       string // displayed to user
	Description string
	Weight      int    // relative time cost (for progress estimation)
	Action      StepAction
}

// StepAction is the function that performs the step.
type StepAction func(installDir string, progress func(pct int, msg string)) error

// Plan is the ordered list of steps to execute.
type Plan struct {
	Steps       []*Step
	InstallDir  string
	NeedsAdmin  bool // Windows UAC
	DiskNeeded  uint64
}

func (p *Plan) NothingToDo() bool { return len(p.Steps) == 0 }

func (p *Plan) TotalWeight() int {
	w := 0
	for _, s := range p.Steps { w += s.Weight }
	return w
}

// BuildPlan inspects the system report and decides what to do.
func BuildPlan(installDir string, report *syscheck.Report) *Plan {
	plan := &Plan{InstallDir: installDir}

	// Step 1: Create directory structure
	plan.Steps = append(plan.Steps, &Step{
		ID:     "dirs",
		Title:  "Creating directories",
		Weight: 1,
		Action: func(dir string, prog func(int, string)) error {
			for _, sub := range []string{"bin", "lib", "share", "config"} {
				if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
					return fmt.Errorf("mkdir %s: %w", sub, err)
				}
			}
			prog(100, "done")
			return nil
		},
	})

	// Step 2: Extract bundled binary
	plan.Steps = append(plan.Steps, &Step{
		ID:     "extract_bin",
		Title:  "Extracting PDFMaster",
		Weight: 20,
		Action: extractBinary,
	})

	// Step 3: Extract bundled libraries (if system ones not found)
	if !report.HasMuPDF || !report.HasQPDF {
		plan.Steps = append(plan.Steps, &Step{
			ID:     "extract_libs",
			Title:  "Extracting PDF libraries",
			Weight: 30,
			Action: extractLibraries,
		})
		plan.DiskNeeded += 25 * 1024 * 1024 // ~25 MB for libs
	} else {
		// System libs found — just symlink/note them
		plan.Steps = append(plan.Steps, &Step{
			ID:     "link_libs",
			Title:  "Linking system libraries",
			Weight: 2,
			Action: linkSystemLibs(report),
		})
	}

	// Step 4: Write config defaults
	plan.Steps = append(plan.Steps, &Step{
		ID:     "config",
		Title:  "Writing default configuration",
		Weight: 1,
		Action: writeDefaultConfig,
	})

	// Step 5: Linux — create .desktop entry
	if report.OS == "linux" {
		plan.Steps = append(plan.Steps, &Step{
			ID:     "desktop",
			Title:  "Creating application shortcut",
			Weight: 1,
			Action: createDesktopEntry,
		})
	}

	// Step 6: Windows — create Start Menu shortcut
	if report.OS == "windows" {
		plan.Steps = append(plan.Steps, &Step{
			ID:     "shortcut",
			Title:  "Creating Start Menu shortcut",
			Weight: 2,
			Action: createWindowsShortcut,
		})
	}

	plan.DiskNeeded += 15 * 1024 * 1024 // binary ~15 MB
	return plan
}

// ExecutePlan runs all steps, calling progressCb(step, pct, msg) for each.
func ExecutePlan(
	plan *Plan,
	installDir string,
	progressCb func(stepIdx, stepTotal int, pct int, msg string),
) error {
	total := len(plan.Steps)
	for i, step := range plan.Steps {
		prog := func(pct int, msg string) {
			if progressCb != nil {
				progressCb(i, total, pct, msg)
			}
		}
		prog(0, "starting")
		if err := step.Action(installDir, prog); err != nil {
			return fmt.Errorf("step %q failed: %w", step.Title, err)
		}
		prog(100, "done")
	}
	return nil
}

// ── Step implementations ──────────────────────────────────────────────

// extractBinary extracts the pdfmaster binary from the embedded payload.
// In the real build this is a go:embed of the pre-built binary.
// The stub here just copies the launcher itself as a placeholder.
func extractBinary(installDir string, prog func(int, string)) error {
	binName := "pdfmaster"
	if runtime.GOOS == "windows" {
		binName = "pdfmaster.exe"
	}
	destPath := filepath.Join(installDir, "bin", binName)
	prog(10, "locating payload")

	// In real build: data is embedded via go:embed
	// Here we simulate by copying the executable itself
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate self: %w", err)
	}

	prog(30, "writing binary")
	if err := copyFile(selfPath, destPath, 0o755); err != nil {
		return fmt.Errorf("cannot write binary: %w", err)
	}
	prog(100, "binary extracted")
	return nil
}

// extractLibraries extracts bundled .so / .dll files.
func extractLibraries(installDir string, prog func(int, string)) error {
	// In a real build these are go:embed'd from packaging/bundled-libs/
	// Stub: create placeholder files so directory structure is correct
	libDir := filepath.Join(installDir, "lib")
	libs := map[string][]string{
		"linux":   {"libmupdf.so.3", "libqpdf.so.29", "libz.so.1", "libpng16.so.16"},
		"windows": {"mupdf.dll", "qpdf29.dll", "zlib1.dll"},
	}
	names := libs[runtime.GOOS]
	for i, lib := range names {
		pct := (i + 1) * 100 / len(names)
		prog(pct, "extracting "+lib)
		// Placeholder empty file — real build writes actual library bytes
		p := filepath.Join(libDir, lib)
		f, err := os.Create(p)
		if err != nil {
			return err
		}
		f.Close()
	}
	return nil
}

func linkSystemLibs(report *syscheck.Report) StepAction {
	return func(installDir string, prog func(int, string)) error {
		prog(50, "system libraries detected")
		// Write a note so we know to skip extraction on repair
		note := filepath.Join(installDir, "lib", ".using-system-libs")
		return os.WriteFile(note, []byte(fmt.Sprintf(
			"mupdf:%v qpdf:%v\n",
			report.HasMuPDF, report.HasQPDF,
		)), 0o644)
	}
}

func writeDefaultConfig(installDir string, prog func(int, string)) error {
	cfgPath := filepath.Join(installDir, "config", "config.toml")
	if _, err := os.Stat(cfgPath); err == nil {
		prog(100, "config exists")
		return nil // don't overwrite existing config
	}
	prog(50, "writing config")
	content := `# PDFMaster configuration (auto-generated)
default_compress_level = 2
default_dpi            = 150
default_jpeg_quality   = 72
default_max_image_dpi  = 150
progress_color         = true
`
	return os.WriteFile(cfgPath, []byte(content), 0o644)
}

func createDesktopEntry(installDir string, prog func(int, string)) error {
	home, _ := os.UserHomeDir()
	desktopDir := filepath.Join(home, ".local", "share", "applications")
	_ = os.MkdirAll(desktopDir, 0o755)

	iconDir := filepath.Join(home, ".local", "share", "icons",
		"hicolor", "256x256", "apps")
	_ = os.MkdirAll(iconDir, 0o755)

	launcherPath, _ := os.Executable()
	desktopContent := fmt.Sprintf(`[Desktop Entry]
Version=1.0
Type=Application
Name=PDFMaster
GenericName=PDF Tool
Comment=View, Merge, Compress and Split PDF files
Exec=%s %%F
Icon=pdfmaster
Terminal=false
Categories=Office;Viewer;
MimeType=application/pdf;
StartupNotify=true
StartupWMClass=PDFMaster
Keywords=PDF;view;merge;compress;split;
`, launcherPath)

	prog(60, "writing .desktop entry")
	entryPath := filepath.Join(desktopDir, "pdfmaster.desktop")
	if err := os.WriteFile(entryPath, []byte(desktopContent), 0o644); err != nil {
		return err
	}

	// Update desktop database if available
	prog(90, "updating desktop database")
	_ = runSilent("update-desktop-database", desktopDir)
	return nil
}

func createWindowsShortcut(installDir string, prog func(int, string)) error {
	prog(50, "creating shortcut")
	// On Windows we'd use a PowerShell one-liner or COM WScript.Shell.
	// Stub: write a .bat launcher to Start Menu instead.
	startMenu := filepath.Join(
		os.Getenv("APPDATA"),
		"Microsoft", "Windows", "Start Menu", "Programs", "PDFMaster")
	_ = os.MkdirAll(startMenu, 0o755)

	launcherPath, _ := os.Executable()
	batContent := fmt.Sprintf(`@echo off
start "" "%s" %%*
`, launcherPath)
	batPath := filepath.Join(startMenu, "PDFMaster.bat")
	return os.WriteFile(batPath, []byte(batContent), 0o644)
}

// ── File utilities ────────────────────────────────────────────────────

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func runSilent(name string, args ...string) error {
	// Import only in this function to avoid top-level dependency
	cmd, err := lookPath(name)
	if err != nil {
		return nil // not found — skip silently
	}
	return runCmd(cmd, args...)
}

// Thin wrappers to avoid importing os/exec at package level (keeps import clean)
var lookPath = func(name string) (string, error) {
	// Resolved at runtime via os/exec — stub for compilation
	return "", fmt.Errorf("not found")
}
var runCmd = func(path string, args ...string) error { return nil }
