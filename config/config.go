package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	dbconfig "github.com/initia-labs/rollytics/orm/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	listenPort      string
	dbConfig        *dbconfig.Config
	chainConfig     *ChainConfig
	logLevel        string
	logFormat       string
	coolingDuration time.Duration // for indexer only
}

func setDefaults() {
	viper.SetDefault("DB_AUTO_MIGRATE", false)
	viper.SetDefault("DB_BATCH_SIZE", 100)
	viper.SetDefault("ACCOUNT_ADDRESS_PREFIX", "init")
	viper.SetDefault("COOLING_DURATION", 100*time.Millisecond)
	viper.SetDefault("LOG_LEVEL", "warn")
	viper.SetDefault("LOG_FORMAT", "plain")
}

func GetConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		// just log without panic, local testing purpose only
		fmt.Fprintln(os.Stderr, "No .env file found")
	}
	viper.AutomaticEnv()
	setDefaults()

	dc := &dbconfig.Config{
		DSN:         viper.GetString("DB_DSN"),
		AutoMigrate: viper.GetBool("DB_AUTO_MIGRATE"),
		MaxConns:    viper.GetInt("DB_MAX_CONNS"),
		IdleConns:   viper.GetInt("DB_IDLE_CONNS"),
		BatchSize:   viper.GetInt("DB_BATCH_SIZE"),
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
		listenPort:      viper.GetString("PORT"),
		dbConfig:        dc,
		chainConfig:     cc,
		logLevel:        viper.GetString("LOG_LEVEL"),
		logFormat:       viper.GetString("LOG_FORMAT"),
		coolingDuration: viper.GetDuration("COOLING_DURATION"),
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

func (c Config) GetListenPort() string {
	return c.listenPort
}

func (c Config) GetDBConfig() *dbconfig.Config {
	return c.dbConfig
}

func (c Config) GetChainConfig() *ChainConfig {
	return c.chainConfig
}

func (c Config) GetDBBatchSize() int {
	return c.dbConfig.BatchSize
}

func (c Config) GetVmType() types.VMType {
	return c.chainConfig.VmType
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
		return slog.LevelDebug
	}
}

func (c Config) GetCoolingDuration() time.Duration {
	return c.coolingDuration
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

	if err := c.dbConfig.Validate(); err != nil {
		return err
	}
	if err := c.chainConfig.Validate(); err != nil {
		return err
	}
	return nil
}
