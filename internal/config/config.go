package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds runtime configuration, all values overridable via environment.
type Config struct {
	Addr          string
	MusicDir      string
	DataDir       string
	DBPath        string
	CoversDir     string
	UploadsDir    string
	WebDir        string
	AdminUser     string
	AdminPassword string
	MaxUploadMB   int64
	ScanOnStart   bool
	// ScanIntervalHours re-scans the library periodically; 0 disables it.
	ScanIntervalHours int
	// GuestMode is on when no initial administrator is configured. The web UI
	// then signs every visitor in as a shared non-admin guest.
	GuestMode bool
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// loadDotEnv applies KEY=VALUE pairs from a local .env file. Real environment
// variables always win, so container/compose configuration is unaffected.
func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

// loadYAML reads the flat key: value subset used by config.yaml. Nested
// structures are intentionally not supported: every option is a single scalar.
func loadYAML(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := make(map[string]string, 16)
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if value != "" && (value[0] == '"' || value[0] == '\'') {
			quote := value[0]
			if end := strings.IndexByte(value[1:], quote); end >= 0 {
				value = value[1 : 1+end]
			} else {
				value = value[1:]
			}
		} else {
			for i := 0; i < len(value); i++ {
				if value[i] == '#' && (i == 0 || value[i-1] == ' ' || value[i-1] == '\t') {
					value = strings.TrimSpace(value[:i])
					break
				}
			}
		}
		out[key] = value
	}
	return out
}

// pick returns the first non-empty value: environment, config.yaml, default.
func pick(y map[string]string, envKey, yamlKey, def string) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	if v := strings.TrimSpace(y[yamlKey]); v != "" {
		return v
	}
	return def
}

// pickRaw distinguishes "explicitly blank" from "not set", which matters for
// the bootstrap administrator: two blank values enable guest mode.
func pickRaw(y map[string]string, envKey, yamlKey, def string) string {
	if v, ok := os.LookupEnv(envKey); ok {
		return strings.TrimSpace(v)
	}
	if v, ok := y[yamlKey]; ok {
		return strings.TrimSpace(v)
	}
	return def
}

func Load() Config {
	loadDotEnv(".env")
	y := loadYAML(env("MROME_CONFIG", "config.yaml"))
	dataDir := pick(y, "MROME_DATA_DIR", "data_dir", "./data")
	c := Config{
		Addr:              pick(y, "MROME_ADDR", "addr", ":4533"),
		MusicDir:          pick(y, "MROME_MUSIC_DIR", "music_dir", filepath.Join(dataDir, "music")),
		DataDir:           dataDir,
		DBPath:            pick(y, "MROME_DB", "db", filepath.Join(dataDir, "mrome.db")),
		CoversDir:         pick(y, "MROME_COVERS_DIR", "covers_dir", filepath.Join(dataDir, "covers")),
		UploadsDir:        pick(y, "MROME_UPLOADS_DIR", "uploads_dir", filepath.Join(dataDir, "uploads")),
		WebDir:            pick(y, "MROME_WEB_DIR", "web_dir", "./web/dist"),
		AdminUser:         pickRaw(y, "MROME_ADMIN_USER", "admin_user", "admin"),
		AdminPassword:     pickRaw(y, "MROME_ADMIN_PASSWORD", "admin_password", "admin"),
		MaxUploadMB:       int64(atoi(pick(y, "MROME_MAX_UPLOAD_MB", "max_upload_mb", "512"), 512)),
		ScanOnStart:       pick(y, "MROME_SCAN_ON_START", "scan_on_start", "true") != "false",
		ScanIntervalHours: atoi(pick(y, "MROME_SCAN_INTERVAL_HOURS", "scan_interval_hours", "6"), 6),
	}
	c.GuestMode = c.AdminUser == "" || c.AdminPassword == ""
	if c.MaxUploadMB < 8 {
		c.MaxUploadMB = 8
	}
	return c
}

func atoi(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func (c Config) EnsureDirs() error {
	for _, d := range []string{c.DataDir, c.MusicDir, c.CoversDir, c.UploadsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
