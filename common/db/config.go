package db

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	DefaultURL     = "postgres://postgres:postgres@localhost:5432/postgres"
	DefaultTimeout = 5 * time.Minute
)

const (
	flagURL     = "db.url"
	flagTimeout = "db.timeout"
)

type Config struct {
	URL     string        `mapstructure:"url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

func NewConfig(url string, timeout time.Duration) *Config {
	return &Config{
		URL:     url,
		Timeout: timeout,
	}
}

func DefaultConfig() *Config {
	return NewConfig(DefaultURL, DefaultTimeout)
}

func (c *Config) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("database URL is required")
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	return nil
}

func GetConfig(v *viper.Viper) (*Config, error) {
	cfg := &Config{
		URL:     v.GetString(flagURL),
		Timeout: v.GetDuration(flagTimeout),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// AddConfigFlags implements servertypes.MoveConfigFlags interface.
func AddConfigFlags(startCmd *cobra.Command) {
	startCmd.Flags().String(flagURL, DefaultURL, "Set the database URL")
	startCmd.Flags().Duration(flagTimeout, DefaultTimeout, "Set the query timeout")
}

// DefaultConfigTemplate default config template for move module
const DefaultConfigTemplate = `
###########################################################
###                         DB                          ###
###########################################################

[db]

# The database URL with the following variables:
#
#   - pool_max_conns: integer greater than 0 (default 4)
#   - pool_min_conns: integer 0 or greater (default 0)
#   - pool_max_conn_lifetime: duration string (default 1 hour)
#   - pool_max_conn_idle_time: duration string (default 30 minutes)
#   - pool_health_check_period: duration string (default 1 minute)
#   - pool_max_conn_lifetime_jitter: duration string (default 0)
#
# See Config for definitions of these arguments.
#
#	# Example Keyword/Value
#	user=jack password=secret host=pg.example.com port=5432 dbname=mydb sslmode=verify-ca pool_max_conns=10 pool_max_conn_lifetime=1h30m
#
#	# Example URL
#	postgres://jack:secret@pg.example.com:5432/mydb?sslmode=verify-ca&pool_max_conns=10&pool_max_conn_lifetime=1h30m
url = "{{ .DBConfig.URL }}"

# The DB operation timeout.
timeout = "{{ .DBConfig.Timeout }}"
`
