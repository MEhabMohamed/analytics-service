package helpers

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"quickbite/analytics-service/lib/config"
)

// LoadTestConfig loads analytics-service/.env.test into process env (overriding existing keys)
// so test runs always target the isolated test MongoDB database and test secrets.
func LoadTestConfig(t *testing.T) *config.Config {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller path")
	}
	rootDir, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve rootDir: %v", err)
	}
	envPath := filepath.Join(rootDir, ".env.test")

	f, err := os.Open(envPath)
	if err != nil {
		t.Fatalf("open .env.test at %s: %v", envPath, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key := strings.TrimSpace(k)
		val := strings.Trim(strings.TrimSpace(v), `"'`)
		_ = os.Setenv(key, val)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read .env.test: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	return cfg
}
