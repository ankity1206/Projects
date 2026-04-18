// launcher/internal/syscheck/syscheck.go
//
// Probes the system for everything PDFMaster needs.
// Runs in < 20 ms; no external process calls on the fast path.

package syscheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Report describes what the system has and what it lacks.
type Report struct {
	OS           string // "linux" | "windows" | "darwin"
	Arch         string // "amd64" | "arm64"
	KernelVer    string
	GlibcVer     string // Linux only; "" if musl or Windows
	HasMuPDF     bool
	MuPDFVersion string
	HasQPDF      bool
	QPDFVersion  string
	HasZlib      bool
	HasGTK3      bool   // Linux: for file-picker integration
	DiskFreeBytes uint64 // in install dir's filesystem
	IsRoot        bool
	HomeDir       string
	TempDir       string
	Warnings      []string
}

// Run probes the system and returns a Report.
func Run() *Report {
	r := &Report{
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		HomeDir: homeDir(),
		TempDir: os.TempDir(),
		IsRoot:  os.Getuid() == 0,
	}

	switch r.OS {
	case "linux":
		r.GlibcVer   = probeGlibc()
		r.KernelVer  = probeKernel()
		r.HasMuPDF   = probeLib("libmupdf.so")   || probeLib("libmupdf.so.3")
		r.HasQPDF    = probeLib("libqpdf.so")     || probeLib("libqpdf.so.29")
		r.HasZlib    = probeLib("libz.so")        || probeLib("libz.so.1")
		r.HasGTK3    = probeLib("libgtk-3.so.0")
		r.MuPDFVersion = probePkgConfig("mupdf")
		r.QPDFVersion  = probePkgConfig("libqpdf")

	case "windows":
		r.HasMuPDF = probeDLL("mupdf.dll") || probeDLL("libmupdf.dll")
		r.HasQPDF  = probeDLL("qpdf29.dll") || probeDLL("libqpdf.dll")
		r.HasZlib  = probeDLL("zlib1.dll") || probeDLL("zlib.dll")
	}

	r.DiskFreeBytes = diskFree(r.HomeDir)

	// Warn if < 150 MB free
	if r.DiskFreeBytes < 150*1024*1024 {
		r.Warnings = append(r.Warnings,
			"Less than 150 MB free disk space — installation may fail")
	}
	return r
}

// ── Probe helpers ─────────────────────────────────────────────────────

func probeLib(name string) bool {
	// Check standard library paths
	searchDirs := []string{
		"/usr/lib", "/usr/lib/x86_64-linux-gnu",
		"/usr/lib/aarch64-linux-gnu",
		"/usr/local/lib", "/lib", "/lib64",
		"/lib/x86_64-linux-gnu",
	}
	// Also check LD_LIBRARY_PATH
	if ldp := os.Getenv("LD_LIBRARY_PATH"); ldp != "" {
		searchDirs = append(searchDirs, strings.Split(ldp, ":")...)
	}
	for _, dir := range searchDirs {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func probeDLL(name string) bool {
	searchDirs := []string{
		`C:\Windows\System32`,
		`C:\Windows\SysWOW64`,
	}
	if path := os.Getenv("PATH"); path != "" {
		searchDirs = append(searchDirs, strings.Split(path, ";")...)
	}
	for _, dir := range searchDirs {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func probePkgConfig(pkg string) string {
	out, err := exec.Command("pkg-config", "--modversion", pkg).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func probeGlibc() string {
	// Read /lib/x86_64-linux-gnu/libc.so.6 output
	out, err := exec.Command("/lib/x86_64-linux-gnu/libc.so.6").Output()
	if err != nil {
		out, err = exec.Command("/lib64/libc.so.6").Output()
	}
	if err != nil {
		return ""
	}
	// Parse "GNU C Library (Ubuntu GLIBC 2.35-0ubuntu3.6) stable..."
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "GLIBC") || strings.Contains(line, "glibc") {
			// Extract version number
			parts := strings.Fields(line)
			for _, p := range parts {
				if len(p) > 0 && (p[0] >= '0' && p[0] <= '9') {
					return p
				}
			}
		}
	}
	return ""
}

func probeKernel() string {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func diskFree(path string) uint64 {
	// Use df as a portable approach
	out, err := exec.Command("df", "-B1", "--output=avail", path).Output()
	if err != nil {
		return 1024 * 1024 * 1024 // assume 1GB if unknown
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 1024 * 1024 * 1024
	}
	n, err := strconv.ParseUint(strings.TrimSpace(lines[1]), 10, 64)
	if err != nil {
		return 1024 * 1024 * 1024
	}
	return n
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.TempDir()
}
