package config

import (
	"os"
	"strings"
	"testing"
)

// createTempConfig is a helper to write a YAML string to a temporary file for testing.
func createTempConfig(t *testing.T, content string) string {
	t.Helper()
	tempFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	if _, err := tempFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp config file: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatalf("Failed to close temp config file: %v", err)
	}
	return tempFile.Name()
}

// TestLoadConfig_Success checks that a valid config file is parsed and processed correctly.
func TestLoadConfig_Success(t *testing.T) {
	validYAML := `
settings:
  listen_port: 8080
  path: "/test/path"
  metric_exporter: true
queues:
  - name: "queue-1"
  - name: "queue-2"
    kind: "fifo"
tokens:
  - name: "admin-user"
    token: "admin-token-is-long-enough-for-validation"
    admin: true
  - name: "rw-user"
    token: "rw-token-is-long-enough-for-validation"
    admin: false
    post: ["queue-1"]
    get: ["queue-2"]
`
	configPath := createTempConfig(t, validYAML)
	cfg, err := LoadConfig(configPath)

	if err != nil {
		t.Fatalf("LoadConfig failed with valid config: %v", err)
	}

	// Check settings (one default, one override)
	if cfg.Settings.ListenAddr != "127.0.0.1" { // Check default
		t.Errorf("expected ListenAddr default '127.0.0.1', got '%s'", cfg.Settings.ListenAddr)
	}
	if cfg.Settings.ListenPort != 8080 { // Check override
		t.Errorf("expected ListenPort 8080, got %d", cfg.Settings.ListenPort)
	}
	if cfg.Settings.MetricExporter != true { // Check override
		t.Errorf("expected MetricExporter to be true")
	}

	// Check queues
	if len(cfg.Queues) != 2 {
		t.Fatalf("expected 2 queues, got %d", len(cfg.Queues))
	}
	if cfg.Queues[0].Name != "queue-1" {
		t.Errorf("expected queue 1 name 'queue-1', got '%s'", cfg.Queues[0].Name)
	}
	if cfg.Queues[0].Kind != "fifo" { // Check default
		t.Errorf("expected queue 1 kind default 'fifo', got '%s'", cfg.Queues[0].Kind)
	}

	// Check TokenMap
	if len(cfg.TokenMap) != 2 {
		t.Fatalf("expected TokenMap to have 2 entries, got %d", len(cfg.TokenMap))
	}

	// Check admin token permissions
	adminPerms, ok := cfg.TokenMap["admin-token-is-long-enough-for-validation"]
	if !ok {
		t.Fatal("admin token not found in TokenMap")
	}
	if !adminPerms.IsAdmin {
		t.Error("admin token IsAdmin flag not set")
	}
	if len(adminPerms.CanPost) != 0 || len(adminPerms.CanGet) != 0 {
		t.Error("admin token should not have granular permissions populated")
	}

	// Check non-admin token permissions
	rwPerms, ok := cfg.TokenMap["rw-token-is-long-enough-for-validation"]
	if !ok {
		t.Fatal("rw token not found in TokenMap")
	}
	if rwPerms.IsAdmin {
		t.Error("rw token IsAdmin flag should be false")
	}
	if !rwPerms.CanPost["queue-1"] {
		t.Error("rw token missing 'post' permission for 'queue-1'")
	}
	if !rwPerms.CanGet["queue-2"] {
		t.Error("rw token missing 'get' permission for 'queue-2'")
	}
	if rwPerms.CanPost["queue-2"] {
		t.Error("rw token has unexpected 'post' permission for 'queue-2'")
	}
}

// TestLoadConfig_Validation checks all failure cases for config loading.
func TestLoadConfig_Validation(t *testing.T) {
	testCases := []struct {
		name      string
		config    string // YAML content
		path      string // Override path (for file-not-found test)
		expectErr string // Substring of the expected error
	}{
		{
			name:      "File not found",
			path:      "non-existent-file.yaml",
			expectErr: "failed to read config file",
		},
		{
			name:      "Invalid YAML syntax",
			config:    `settings: [invalid`,
			expectErr: "failed to parse YAML config",
		},
		{
			name: "No queues",
			config: `
settings:
  path: /tmp
  listenPort: 1
tokens:
  - name: t
    token: "long-enough-token-string"
`,
			expectErr: "'queues' list cannot be empty",
		},
		{
			name: "No tokens",
			config: `
settings:
  path: /tmp
  listenPort: 1
queues:
  - name: q1
`,
			expectErr: "'tokens' list cannot be empty",
		},
		{
			name: "Invalid queue name (regex)",
			config: `
settings:
  path: /tmp
queues:
  - name: "invalid name!"
tokens:
  - name: t
    token: "long-enough-token-string"
`,
			expectErr: "name can only contain letters,digits,hyphens",
		},
		{
			name: "Empty queue name",
			config: `
settings:
  path: /tmp
queues:
  - name: ""
tokens:
  - name: t
    token: "long-enough-token-string"
`,
			expectErr: "'queue #0' requires a name",
		},
		{
			name: "Bad queue kind",
			config: `
settings:
  path: /tmp
queues:
  - name: "test"
    kind: "bad"
tokens:
  - name: t
    token: "long-enough-token-string"
`,
			expectErr: "'queue #0 test' is of unsupported kind",
		},
		{
			name: "Duplicate token string",
			config: `
settings:
  path: /tmp
queues:
  - name: q1
tokens:
  - name: t1
    token: "duplicate-token-string-long-enough"
  - name: t2
    token: "duplicate-token-string-long-enough"
`,
			expectErr: "is duplicated",
		},
		{
			name: "Token too short",
			config: `
settings:
  path: /tmp
queues:
  - name: q1
tokens:
  - name: t1
    token: "short"
`,
			expectErr: "token should be at least 20 characters",
		},
		{
			name: "Empty token name",
			config: `
settings:
  path: /tmp
queues:
  - name: q1
tokens:
  - name: ""
    token: "long-enough-token-string"
`,
			expectErr: "'token #0' requires a name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := tc.path
			if configPath == "" {
				// Create the temp file if content is provided
				configPath = createTempConfig(t, tc.config)
			}

			_, err := LoadConfig(configPath)

			if err == nil {
				t.Fatal("expected an error but got nil")
			}
			if !strings.Contains(err.Error(), tc.expectErr) {
				t.Errorf("expected error to contain '%s', but got: %v", tc.expectErr, err)
			}
		})
	}
}
