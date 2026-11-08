// Package config parses and validates environment variables into a strongly-typed struct.
//
// TS vs Go Syntax Note:
//   - In `order-service/src/lib/config/env.ts`, `dotenv` loads `.env` and a `zod` schema validates `process.env`.
//   - In Go, `github.com/caarlos0/env/v11` reads struct tags (`env:"PORT" envDefault:"5000"`,
//     `env:"ACCESS_SECRET,required"`, `envSeparator:","`) and populates `Config` fields (including
//     `int` and `[]string` slices) automatically without manual `Number(...)` or `.split(",")` calls.
package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config holds all parsed and validated environment variables for analytics-service.
// Equivalent to the zod schema in order-service/src/lib/config/env.ts.
type Config struct {
	Port int    `env:"PORT" envDefault:"5000"`
	Env  string `env:"ENV" envDefault:"development"`

	MongoURI  string `env:"MONGO_URI" envDefault:"mongodb+srv://<db_username>:<db_password>@quickbite.nlf5hj9.mongodb.net/"`
	MongoDB   string `env:"MONGO_DB" envDefault:"quickbite_analytics"`
	MongoUser string `env:"MONGO_USER"`
	MongoPass string `env:"MONGO_PASS"`

	RabbitURL string `env:"RABBITMQ_URL" envDefault:"amqp://guest:guest@localhost:5672/"`

	OrderEventsExchange string   `env:"RABBITMQ_ORDER_EVENTS_EXCHANGE" envDefault:"order.events"`
	OrderEventsQueue    string   `env:"RABBITMQ_ORDER_EVENTS_QUEUE" envDefault:"analytics-service.order-events"`
	OrderEventsBindings []string `env:"RABBITMQ_ORDER_EVENTS_BINDINGS" envSeparator:"," envDefault:"order.#,payment.#"`
	OrderEventsDLX      string   `env:"RABBITMQ_ORDER_EVENTS_DLX" envDefault:"order.events.dlx"`
	OrderEventsDLQ      string   `env:"RABBITMQ_ORDER_EVENTS_DLQ" envDefault:"analytics-service.order-events.dlq"`

	CoreEventsExchange string   `env:"RABBITMQ_CORE_EVENTS_EXCHANGE" envDefault:"core.events"`
	CoreEventsQueue    string   `env:"RABBITMQ_CORE_EVENTS_QUEUE" envDefault:"analytics-service.core-events"`
	CoreEventsBindings []string `env:"RABBITMQ_CORE_EVENTS_BINDINGS" envSeparator:"," envDefault:"rbac.#"`
	CoreEventsDLX      string   `env:"RABBITMQ_CORE_EVENTS_DLX" envDefault:"core.events.dlx"`
	CoreEventsDLQ      string   `env:"RABBITMQ_CORE_EVENTS_DLQ" envDefault:"analytics-service.core-events.dlq"`

	RabbitPrefetch int `env:"RABBITMQ_PREFETCH" envDefault:"32"`

	AccessSecret   string `env:"ACCESS_SECRET,required"`
	InternalAPIKey string `env:"INTERNAL_API_KEY" envDefault:"internal-secret-api-key"`

	CoreServiceBaseURL string `env:"CORE_SERVICE_BASE_URL" envDefault:"http://localhost:3001"`
	CoreInternalAPIKey string `env:"CORE_INTERNAL_API_KEY" envDefault:"internal-secret-api-key"`
	RBACCacheTTLSec    int    `env:"RBAC_CACHE_TTL_SEC" envDefault:"300"`

	BackfillPGURLs string `env:"BACKFILL_PG_URLS"`
}

// Load reads .env (if present in working dir) into missing process env vars and parses Config via caarlos0/env/v11.
func Load() (*Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return nil, fmt.Errorf("read .env: %w", err)
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Dynamically interpolate <db_username> and <db_password> in MONGO_URI from MONGO_USER / MONGO_PASS
	// (URL-escaping credentials so special characters in passwords never break URI parsing).
	if cfg.MongoUser != "" {
		escapedUser := url.QueryEscape(cfg.MongoUser)
		cfg.MongoURI = strings.ReplaceAll(cfg.MongoURI, "<db_username>", escapedUser)
		cfg.MongoURI = strings.ReplaceAll(cfg.MongoURI, "${MONGO_USER}", escapedUser)
	}
	if cfg.MongoPass != "" {
		escapedPass := url.QueryEscape(cfg.MongoPass)
		cfg.MongoURI = strings.ReplaceAll(cfg.MongoURI, "<db_password>", escapedPass)
		cfg.MongoURI = strings.ReplaceAll(cfg.MongoURI, "${MONGO_PASS}", escapedPass)
	}

	return &cfg, nil
}

// loadDotEnv populates unset environment variables from a local .env file if it exists,
// walking up parent directories if .env is not in the current working directory.
//
// TS vs Go Syntax Note (Iterator Error Checking):
//   - `for scanner.Scan()` returns `false` both when it reaches EOF normally AND when a read I/O error
//     (e.g., line exceeding buffer size or disk error) occurs.
//   - Therefore, Go's `bufio.Scanner` (and `sql.Rows`) pattern requires checking `scanner.Err()`
//     immediately after the loop exits to distinguish clean EOF (`nil`) from a read failure.
func loadDotEnv(filename string) error {
	path := findDotEnv(filename)
	if path == "" {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}

	return scanner.Err()
}

// findDotEnv searches the current directory and up to 4 parent directories for filename.
func findDotEnv(filename string) string {
	dir, err := os.Getwd()
	if err != nil {
		return filename
	}
	for i := 0; i <= 4; i++ {
		candidate := filepath.Join(dir, filename)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

