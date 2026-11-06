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
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config holds all parsed and validated environment variables for analytics-service.
// Equivalent to the zod schema in order-service/src/lib/config/env.ts.
type Config struct {
	Port int    `env:"PORT" envDefault:"5000"`
	Env  string `env:"ENV" envDefault:"development"`

	MongoURI string `env:"MONGO_URI" envDefault:"mongodb://localhost:27017"`
	MongoDB  string `env:"MONGO_DB" envDefault:"quickbite_analytics"`

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
	loadDotEnv(".env")

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// loadDotEnv populates unset environment variables from a local .env file if it exists.
func loadDotEnv(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return
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
}

