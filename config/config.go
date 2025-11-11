package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
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

// Default configuration constants
const (
	// Port settings
	DefaultAPIPort     = "8080"
	DefaultMetricsPort = "9090"
	MinPortNumber      = 1
	MaxPortNumber      = 65535

	// Database settings
	DefaultDBMaxConns  = 0 // 0 means unlimited (GORM default)
	DefaultDBIdleConns = 2 // GORM default
	DefaultDBBatchSize = 100

	// Cache settings
	DefaultCacheSize = 1000
	DefaultCacheTTL  = 10 * time.Minute

	// Dictionary cache settings
	DefaultAccountCacheSize          = 40960
	DefaultNftCacheSize              = 40960
	DefaultMsgTypeCacheSize          = 1024
	DefaultTypeTagCacheSize          = 1024
	DefaultEvmTxHashCacheSize        = 40960
	DefaultEvmDenomContractCacheSize = 10240

	// Timeout and interval settings
	DefaultCoolingDuration = 50 * time.Millisecond
	DefaultQueryTimeout    = 30 * time.Second
	DefaultPollingInterval = 3 * time.Second

	// Concurrent request settings
	DefaultMaxConcurrentRequests = 50
	MaxAllowedConcurrentRequests = 1000

	// Internal TX settings
	DefaultInternalTxPollInterval = 5 * time.Second
	DefaultInternalTxBatchSize    = 10
	DefaultInternalTxQueueSize    = 100 // Default queue size

	// Metrics settings
	DefaultMetricsPath = "/metrics"

	// Default address prefix
	DefaultAccountAddressPrefix = "init"

	// Default environment
	DefaultEnvironment = "local"
)

type MetricsConfig struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
	Port    string `json:"port"`
}

// CacheConfig contains configuration for dictionary caches
type CacheConfig struct {
	AccountCacheSize          int `json:"account_cache_size"`
	NftCacheSize              int `json:"nft_cache_size"`
	MsgTypeCacheSize          int `json:"msg_type_cache_size"`
	TypeTagCacheSize          int `json:"type_tag_cache_size"`
	EvmTxHashCacheSize        int `json:"evm_tx_hash_cache_size"`
	EvmDenomContractCacheSize int `json:"evm_denom_contract_cache_size"`
}

// SentryConfig contains configuration for Sentry integration
type SentryConfig struct {
	DSN                string  `json:"dsn"`
	SampleRate         float64 `json:"sample_rate"`          // General sample rate (fallback)
	TracesSampleRate   float64 `json:"traces_sample_rate"`   // Traces sample rate
	ProfilesSampleRate float64 `json:"profiles_sample_rate"` // Profiles sample rate
	Environment        string  `json:"environment"`
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
	richListConfig        *RichListConfig
	metricsConfig         *MetricsConfig
	cacheConfig           *CacheConfig
	sentryConfig          *SentryConfig

	// Start height configuration
	startHeight    int64 // explicit start height when set
	startHeightSet bool  // whether START_HEIGHT was provided
}

func setDefaults() {
	viper.SetDefault("PORT", DefaultAPIPort)
	viper.SetDefault("DB_AUTO_MIGRATE", false)
	viper.SetDefault("DB_BATCH_SIZE", DefaultDBBatchSize)
	viper.SetDefault("DB_MAX_CONNS", DefaultDBMaxConns)
	viper.SetDefault("DB_IDLE_CONNS", DefaultDBIdleConns)
	viper.SetDefault("DB_MIGRATION_DIR", "orm/migrations")
	viper.SetDefault("ACCOUNT_ADDRESS_PREFIX", DefaultAccountAddressPrefix)
	viper.SetDefault("COOLING_DURATION", DefaultCoolingDuration)
	viper.SetDefault("QUERY_TIMEOUT", DefaultQueryTimeout)
	viper.SetDefault("MAX_CONCURRENT_REQUESTS", DefaultMaxConcurrentRequests)
	viper.SetDefault("LOG_LEVEL", "warn")
	viper.SetDefault("LOG_FORMAT", "json")
	viper.SetDefault("CACHE_SIZE", DefaultCacheSize)
	viper.SetDefault("CACHE_TTL", DefaultCacheTTL)
	viper.SetDefault("POLLING_INTERVAL", DefaultPollingInterval)
	viper.SetDefault("INTERNAL_TX_POLL_INTERVAL", DefaultInternalTxPollInterval)
	viper.SetDefault("INTERNAL_TX_BATCH_SIZE", DefaultInternalTxBatchSize)
	viper.SetDefault("INTERNAL_TX_QUEUE_SIZE", DefaultInternalTxQueueSize)
	viper.SetDefault("RICH_LIST", true)
	viper.SetDefault("METRICS_ENABLED", false)
	viper.SetDefault("METRICS_PATH", DefaultMetricsPath)
	viper.SetDefault("METRICS_PORT", DefaultMetricsPort)
	viper.SetDefault("ENVIRONMENT", DefaultEnvironment)

	// Sentry defaults
	viper.SetDefault("SENTRY_DSN", "")
	viper.SetDefault("SENTRY_SAMPLE_RATE", 0.01)
	viper.SetDefault("SENTRY_TRACES_SAMPLE_RATE", 0.01)
	viper.SetDefault("SENTRY_PROFILES_SAMPLE_RATE", 0.01)

	// Dictionary cache defaults
	viper.SetDefault("ACCOUNT_CACHE_SIZE", DefaultAccountCacheSize)
	viper.SetDefault("NFT_CACHE_SIZE", DefaultNftCacheSize)
	viper.SetDefault("MSG_TYPE_CACHE_SIZE", DefaultMsgTypeCacheSize)
	viper.SetDefault("TYPE_TAG_CACHE_SIZE", DefaultTypeTagCacheSize)
	viper.SetDefault("EVM_TX_HASH_CACHE_SIZE", DefaultEvmTxHashCacheSize)
	viper.SetDefault("EVM_DENOM_CONTRACT_CACHE_SIZE", DefaultEvmDenomContractCacheSize)

	//  CHAIN_ID, VM_TYPE, RPC_URL, REST_URL, and JSON_RPC_URL have no defaults
}

// setVMSpecificDefaults sets defaults based on VM type
func setVMSpecificDefaults(vmType types.VMType) {
	switch vmType {
	case types.EVM:
		viper.SetDefault("INTERNAL_TX", true)
	default:
		viper.SetDefault("INTERNAL_TX", false)
	}
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

	var vmType types.VMType
	switch viper.GetString("VM_TYPE") {
	case "move":
		vmType = types.MoveVM
	case "wasm":
		vmType = types.WasmVM
	case "evm":
		vmType = types.EVM
	default:
		return nil, types.NewConfigError("VM_TYPE is invalid", nil)
	}

	// Set VM-specific defaults before general defaults
	setVMSpecificDefaults(vmType)
	setDefaults()

	dc := &dbconfig.Config{
		DSN:          viper.GetString("DB_DSN"),
		AutoMigrate:  viper.GetBool("DB_AUTO_MIGRATE"),
		MaxConns:     viper.GetInt("DB_MAX_CONNS"),
		IdleConns:    viper.GetInt("DB_IDLE_CONNS"),
		BatchSize:    viper.GetInt("DB_BATCH_SIZE"),
		MigrationDir: viper.GetString("DB_MIGRATION_DIR"),
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
			QueueSize:    viper.GetInt("INTERNAL_TX_QUEUE_SIZE"),
		},
		richListConfig: &RichListConfig{
			Enabled: viper.GetBool("RICH_LIST"),
		},
		metricsConfig: &MetricsConfig{
			Enabled: viper.GetBool("METRICS_ENABLED"),
			Path:    viper.GetString("METRICS_PATH"),
			Port:    viper.GetString("METRICS_PORT"),
		},
		cacheConfig: &CacheConfig{
			AccountCacheSize:          viper.GetInt("ACCOUNT_CACHE_SIZE"),
			NftCacheSize:              viper.GetInt("NFT_CACHE_SIZE"),
			MsgTypeCacheSize:          viper.GetInt("MSG_TYPE_CACHE_SIZE"),
			TypeTagCacheSize:          viper.GetInt("TYPE_TAG_CACHE_SIZE"),
			EvmTxHashCacheSize:        viper.GetInt("EVM_TX_HASH_CACHE_SIZE"),
			EvmDenomContractCacheSize: viper.GetInt("EVM_DENOM_CONTRACT_CACHE_SIZE"),
		},
		sentryConfig: &SentryConfig{
			DSN:                viper.GetString("SENTRY_DSN"),
			SampleRate:         viper.GetFloat64("SENTRY_SAMPLE_RATE"),
			TracesSampleRate:   viper.GetFloat64("SENTRY_TRACES_SAMPLE_RATE"),
			ProfilesSampleRate: viper.GetFloat64("SENTRY_PROFILES_SAMPLE_RATE"),
			Environment:        viper.GetString("ENVIRONMENT"),
		},
	}

	// parse optional START_HEIGHT env var. Accepts integer >= 0.
	raw := strings.TrimSpace(viper.GetString("START_HEIGHT"))
	if raw == "" {
		// not set; do nothing
	} else {
		val, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || val < 0 {
			return nil, types.NewInvalidValueError("START_HEIGHT", raw, "must be a non-negative integer")
		}
		config.startHeight = val
		config.startHeightSet = true
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

func (c Config) GetRichListEnabled() bool {
	if c.richListConfig == nil {
		return false
	}
	return c.richListConfig.Enabled
}

func (c Config) GetRichListConfig() *RichListConfig {
	return c.richListConfig
}

func (c Config) GetSentryConfig() *SentryConfig {
	if c.sentryConfig == nil || c.sentryConfig.DSN == "" {
		return nil
	}
	return c.sentryConfig
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

func (c Config) GetCacheConfig() *CacheConfig {
	return c.cacheConfig
}

func (c Config) GetLogFormat() string {
	if c.logFormat == "json" {
		return "json"
	}
	return "plain"
}

// Start height accessors
func (c Config) StartHeightSet() bool {
	return c.startHeightSet
}

func (c Config) GetStartHeight() int64 {
	return c.startHeight
}

func (c Config) Validate() error {
	if err := c.validatePort(); err != nil {
		return err
	}
	if err := c.validateLogSettings(); err != nil {
		return err
	}
	if err := c.validateNumericSettings(); err != nil {
		return err
	}
	if err := c.validateInternalTxConfig(); err != nil {
		return err
	}
	if err := c.validateMetricsConfig(); err != nil {
		return err
	}
	if err := c.validateSubConfigs(); err != nil {
		return err
	}
	return nil
}

// validatePort validates the listen port configuration
func (c Config) validatePort() error {
	if len(c.listenPort) == 0 {
		return types.NewValidationError("PORT", "required field is missing")
	}
	if port, err := strconv.Atoi(c.listenPort); err != nil || port < MinPortNumber || port > MaxPortNumber {
		return types.NewValidationError("PORT", fmt.Sprintf("must be a valid port number (%d-%d)", MinPortNumber, MaxPortNumber))
	}
	return nil
}

// validateLogSettings validates log format and level configuration
func (c Config) validateLogSettings() error {
	// Log format validation
	switch c.logFormat {
	case "json", "plain":
		break
	default:
		return types.NewValidationError("LOG_FORMAT", fmt.Sprintf("invalid value '%s', must be 'json' or 'plain'", c.logFormat))
	}

	// Log level validation
	switch c.logLevel {
	case "debug", "info", "warn", "error":
		break
	default:
		return types.NewValidationError("LOG_LEVEL", fmt.Sprintf("invalid value '%s', must be one of: debug, info, warn, error", c.logLevel))
	}
	return nil
}

// validateNumericSettings validates all numeric configuration values
func (c Config) validateNumericSettings() error {
	if c.cacheSize < 0 {
		return types.NewValidationError("CACHE_SIZE", "must be non-negative")
	}
	if c.cacheTTL < 0 {
		return types.NewValidationError("CACHE_TTL", "must be non-negative")
	}
	if c.pollingInterval < 0 {
		return types.NewValidationError("POLLING_INTERVAL", "must be non-negative")
	}
	if c.coolingDuration < 0 {
		return types.NewValidationError("COOLING_DURATION", "must be non-negative")
	}
	if c.queryTimeout <= 0 {
		return types.NewValidationError("QUERY_TIMEOUT", "must be positive")
	}
	if c.maxConcurrentRequests < 1 {
		return types.NewValidationError("MAX_CONCURRENT_REQUESTS", "must be at least 1")
	}
	if c.maxConcurrentRequests > MaxAllowedConcurrentRequests {
		return types.NewInvalidValueError("MAX_CONCURRENT_REQUESTS", fmt.Sprintf("%d", c.maxConcurrentRequests), fmt.Sprintf("must not exceed %d", MaxAllowedConcurrentRequests))
	}
	return nil
}

// validateInternalTxConfig validates internal transaction configuration
func (c Config) validateInternalTxConfig() error {
	if c.internalTxConfig != nil && c.internalTxConfig.Enabled {
		if err := c.validateInternalTxVMSupport(); err != nil {
			return err
		}
		if err := c.validateInternalTxSettings(); err != nil {
			return err
		}
		if c.internalTxConfig.QueueSize < 1 {
			return types.NewValidationError("INTERNAL_TX_QUEUE_SIZE", "must be at least 1")
		}
	}
	return nil
}

// validateInternalTxVMSupport validates that internal TX is only enabled for supported VM types
func (c Config) validateInternalTxVMSupport() error {
	if c.chainConfig.VmType != types.EVM {
		vmTypeStr := "unknown"
		switch c.chainConfig.VmType {
		case types.MoveVM:
			vmTypeStr = "Move"
		case types.WasmVM:
			vmTypeStr = "Wasm"
		}
		return types.NewValidationError("INTERNAL_TX", fmt.Sprintf("internal transaction tracking is not supported for %s VM type, only EVM", vmTypeStr))
	}
	return nil
}

// validateInternalTxSettings validates internal transaction specific settings
func (c Config) validateInternalTxSettings() error {
	if c.internalTxConfig.PollInterval <= 0 {
		return types.NewValidationError("INTERNAL_TX_POLL_INTERVAL", "must be positive when INTERNAL_TX is enabled")
	}
	if c.internalTxConfig.BatchSize < 1 {
		return types.NewValidationError("INTERNAL_TX_BATCH_SIZE", "must be at least 1")
	}
	return nil
}

// validateMetricsConfig validates metrics configuration
func (c Config) validateMetricsConfig() error {
	if c.metricsConfig != nil && c.metricsConfig.Enabled {
		if err := c.validateMetricsPort(); err != nil {
			return err
		}
		if err := c.validateMetricsPath(); err != nil {
			return err
		}
	}
	return nil
}

// validateMetricsPort validates the metrics port configuration
func (c Config) validateMetricsPort() error {
	if port, err := strconv.Atoi(c.metricsConfig.Port); err != nil || port < MinPortNumber || port > MaxPortNumber {
		return types.NewValidationError("METRICS_PORT", fmt.Sprintf("must be a valid port number (%d-%d)", MinPortNumber, MaxPortNumber))
	}
	if c.metricsConfig.Port == c.listenPort {
		return types.NewValidationError("METRICS_PORT", fmt.Sprintf("metrics port %s conflicts with API port", c.metricsConfig.Port))
	}
	return nil
}

// validateMetricsPath validates the metrics path configuration
func (c Config) validateMetricsPath() error {
	if c.metricsConfig.Path == "" || c.metricsConfig.Path[0] != '/' {
		return types.NewValidationError("METRICS_PATH", "must start with '/'")
	}
	return nil
}

// validateSubConfigs validates nested configuration objects
func (c Config) validateSubConfigs() error {
	if err := c.dbConfig.Validate(); err != nil {
		return err
	}
	if err := c.chainConfig.Validate(); err != nil {
		return err
	}
	return nil
}
