// internal/config/config.go
// User configuration loaded from ~/.config/pdfmaster/config.toml
// (TOML parsed manually to avoid heavy dependencies).

package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all user-configurable defaults.
type Config struct {
	DefaultCompressLevel int    // 1=light 2=medium 3=heavy
	DefaultDPI           int    // render DPI
	DefaultJpegQuality   int    // 1-100
	DefaultMaxImageDPI   int    // heavy compress threshold
	ProgressColor        bool   // coloured progress bars
	TempDir              string // scratch directory
}

// Default returns the factory default configuration.
func Default() *Config {
	return &Config{
		DefaultCompressLevel: 2,
		DefaultDPI:           150,
		DefaultJpegQuality:   72,
		DefaultMaxImageDPI:   150,
		ProgressColor:        true,
		TempDir:              os.TempDir(),
	}
}

// ConfigPath returns the path to the user config file.
func ConfigPath() string {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(cfgDir, "pdfmaster", "config.toml")
}

// Load reads config from disk.  Missing file → returns Default().
// Parse errors are silently ignored (returns Default()).
func Load() *Config {
	cfg := Default()

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg // file does not exist — use defaults
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		v = strings.Trim(v, `"'`)

		switch k {
		case "default_compress_level":
			if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 3 {
				cfg.DefaultCompressLevel = n
			}
		case "default_dpi":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				cfg.DefaultDPI = n
			}
		case "default_jpeg_quality":
			if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 100 {
				cfg.DefaultJpegQuality = n
			}
		case "default_max_image_dpi":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				cfg.DefaultMaxImageDPI = n
			}
		case "progress_color":
			cfg.ProgressColor = v == "true" || v == "1"
		case "temp_dir":
			if v != "" {
				cfg.TempDir = v
			}
		}
	}
	return cfg
}

// Save writes the current config to disk.
func (c *Config) Save() error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lines := []string{
		"# PDFMaster configuration",
		"# Edit values below and save.",
		"",
		"default_compress_level = " + strconv.Itoa(c.DefaultCompressLevel),
		"default_dpi            = " + strconv.Itoa(c.DefaultDPI),
		"default_jpeg_quality   = " + strconv.Itoa(c.DefaultJpegQuality),
		"default_max_image_dpi  = " + strconv.Itoa(c.DefaultMaxImageDPI),
		"progress_color         = " + strconv.FormatBool(c.ProgressColor),
		"temp_dir               = \"" + c.TempDir + "\"",
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
