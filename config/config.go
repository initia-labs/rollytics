package config

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	dbconfig "github.com/initia-labs/rollytics/orm/config"
	"github.com/initia-labs/rollytics/types"
)

var (
	Version    = "dev"
	CommitHash = "unknown"

	// Singleton instance
	configInstance *Config
	configOnce     sync.Once
)

type MetricsConfig struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
	Port    string `json:"port"`
}

func SetBuildInfo(v, commit string) {
	Version = v
	CommitHash = commit
}

type Config struct {
	listenPort            string
	dbConfig              *dbconfig.Config
	chainConfig           *ChainConfig
	logLevel              string
	logFormat             string
	coolingDuration       time.Duration // for indexer only
	queryTimeout          time.Duration // for indexer only
	maxConcurrentRequests int           // for indexer only
	cacheSize             int
	cacheTTL              time.Duration // for api only
	pollingInterval       time.Duration // for api only
	internalTxConfig      *InternalTxConfig
	metricsConfig         *MetricsConfig
}

func setDefaults() {
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("DB_AUTO_MIGRATE", false)
	viper.SetDefault("DB_BATCH_SIZE", 100)
	viper.SetDefault("DB_MAX_CONNS", 0)  // 0 means unlimited (GORM default)
	viper.SetDefault("DB_IDLE_CONNS", 2) // GORM default is 2
	viper.SetDefault("DB_MIGRATION_DIR", "orm/migrations")
	viper.SetDefault("ACCOUNT_ADDRESS_PREFIX", "init")
	viper.SetDefault("COOLING_DURATION", 100*time.Millisecond)
	viper.SetDefault("QUERY_TIMEOUT", 10*time.Second)
	viper.SetDefault("MAX_CONCURRENT_REQUESTS", 50)
	viper.SetDefault("LOG_LEVEL", "warn")
	viper.SetDefault("LOG_FORMAT", "json")
	viper.SetDefault("CACHE_SIZE", 1000)
	viper.SetDefault("CACHE_TTL", 10*time.Minute)
	viper.SetDefault("POLLING_INTERVAL", 3*time.Second)
	viper.SetDefault("INTERNAL_TX", false)
	viper.SetDefault("INTERNAL_TX_POLL_INTERVAL", 5*time.Second)
	viper.SetDefault("INTERNAL_TX_BATCH_SIZE", 10)
	viper.SetDefault("METRICS_ENABLED", false)
	viper.SetDefault("METRICS_PATH", "/metrics")
	viper.SetDefault("METRICS_PORT", "9090")
	//  CHAIN_ID, VM_TYPE, RPC_URL, REST_URL, and JSON_RPC_URL have no defaults
}

func GetConfig() (*Config, error) {
	var err error

	configOnce.Do(func() {
		configInstance, err = loadConfig()
	})

	return configInstance, err
}

func loadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		// just log without panic, local testing purpose only
		fmt.Fprintln(os.Stderr, "No .env file found")
	}
	viper.AutomaticEnv()
	setDefaults()

	dc := &dbconfig.Config{
		DSN:          viper.GetString("DB_DSN"),
		AutoMigrate:  viper.GetBool("DB_AUTO_MIGRATE"),
		MaxConns:     viper.GetInt("DB_MAX_CONNS"),
		IdleConns:    viper.GetInt("DB_IDLE_CONNS"),
		BatchSize:    viper.GetInt("DB_BATCH_SIZE"),
		MigrationDir: viper.GetString("DB_MIGRATION_DIR"),
	}

	var vmType types.VMType
	switch viper.GetString("VM_TYPE") {
	case "move":
		vmType = types.MoveVM
	case "wasm":
		vmType = types.WasmVM
	case "evm":
		vmType = types.EVM
	default:
		return nil, fmt.Errorf("VM_TYPE is invalid")
	}

	cc := &ChainConfig{
		ChainId:              viper.GetString("CHAIN_ID"),
		VmType:               vmType,
		RpcUrl:               viper.GetString("RPC_URL"),
		RestUrl:              viper.GetString("REST_URL"),
		JsonRpcUrl:           viper.GetString("JSON_RPC_URL"),
		AccountAddressPrefix: viper.GetString("ACCOUNT_ADDRESS_PREFIX"),
	}

	config := &Config{
		listenPort:            viper.GetString("PORT"),
		dbConfig:              dc,
		chainConfig:           cc,
		logLevel:              viper.GetString("LOG_LEVEL"),
		logFormat:             viper.GetString("LOG_FORMAT"),
		coolingDuration:       viper.GetDuration("COOLING_DURATION"),
		queryTimeout:          viper.GetDuration("QUERY_TIMEOUT"),
		maxConcurrentRequests: viper.GetInt("MAX_CONCURRENT_REQUESTS"),
		cacheSize:             viper.GetInt("CACHE_SIZE"),
		cacheTTL:              viper.GetDuration("CACHE_TTL"),
		pollingInterval:       viper.GetDuration("POLLING_INTERVAL"),
		internalTxConfig: &InternalTxConfig{
			Enabled:      viper.GetBool("INTERNAL_TX"),
			PollInterval: viper.GetDuration("INTERNAL_TX_POLL_INTERVAL"),
			BatchSize:    viper.GetInt("INTERNAL_TX_BATCH_SIZE"),
		},
		metricsConfig: &MetricsConfig{
			Enabled: viper.GetBool("METRICS_ENABLED"),
			Path:    viper.GetString("METRICS_PATH"),
			Port:    viper.GetString("METRICS_PORT"),
		},
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	// initialize sdk
	InitializeSDKConfig(cc.AccountAddressPrefix)

	return config, nil
}

func (c Config) GetListenPort() string {
	return c.listenPort
}

// SetDBConfig assigns the DB config for testing purposes.
func (c *Config) SetDBConfig(dbCfg *dbconfig.Config) {
	c.dbConfig = dbCfg
}

func (c Config) GetDBConfig() *dbconfig.Config {
	return c.dbConfig
}

// SetChainConfig assigns the chain config for testing purposes.
func (c *Config) SetChainConfig(chainCfg *ChainConfig) {
	c.chainConfig = chainCfg
}

func (c Config) GetChainConfig() *ChainConfig {
	return c.chainConfig
}

// SetInternalTxConfig assigns the internal tx config for testing purposes.
func (c *Config) SetInternalTxConfig(internalTxCfg *InternalTxConfig) {
	c.internalTxConfig = internalTxCfg
}

func (c Config) GetDBBatchSize() int {
	return c.dbConfig.BatchSize
}

func (c Config) GetCacheSize() int {
	return c.cacheSize
}

func (c Config) GetCacheTTL() time.Duration {
	return c.cacheTTL
}

func (c Config) GetPollingInterval() time.Duration {
	return c.pollingInterval
}

func (c Config) GetChainId() string {
	return c.chainConfig.ChainId
}

func (c Config) GetVmType() types.VMType {
	return c.chainConfig.VmType
}

func (c Config) InternalTxEnabled() bool {
	return c.internalTxConfig.Enabled
}

func (c Config) GetInternalTxConfig() *InternalTxConfig {
	return c.internalTxConfig
}

func (c Config) GetLogLevel() slog.Level {
	switch c.logLevel {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

func (c Config) GetCoolingDuration() time.Duration {
	return c.coolingDuration
}

func (c Config) GetQueryTimeout() time.Duration {
	return c.queryTimeout
}

func (c Config) GetMaxConcurrentRequests() int {
	return c.maxConcurrentRequests
}

func (c Config) GetMetricsConfig() *MetricsConfig {
	return c.metricsConfig
}

func (c Config) GetLogFormat() string {
	if c.logFormat == "json" {
		return "json"
	}
	return "plain"
}

func (c Config) Validate() error {
	if len(c.listenPort) == 0 {
		return fmt.Errorf("PORT is required")
	}
	switch c.logFormat {
	case "json", "plain":
		break
	default:
		return fmt.Errorf("%s is invalid log format", c.logFormat)
	}

	if c.cacheSize < 0 {
		return fmt.Errorf("CACHE_SIZE must be non-negative")
	}
	if c.cacheTTL < 0 {
		return fmt.Errorf("CACHE_TTL must be non-negative")
	}
	if c.pollingInterval < 0 {
		return fmt.Errorf("POLLING_INTERVAL must be non-negative")
	}
	if err := c.dbConfig.Validate(); err != nil {
		return err
	}
	if err := c.chainConfig.Validate(); err != nil {
		return err
	}
	return nil
}
