package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"

	dbconfig "github.com/initia-labs/rollytics/orm/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// api global config
type Config struct {
	listenAddr  string
	dbConfig    *dbconfig.Config
	chainConfig *ChainConfig
	logLevel    string
}

type ChainConfig struct {
	ChainId    string
	VmType     types.VMType
	RpcUrl     string
	RestUrl    string
	JsonRpcUrl string
}

func setDefaults() {
	viper.SetDefault("ENABLE_PROFILE", false)
	viper.SetDefault("LISTEN_ADDR", ":8080")
	viper.SetDefault("DB_BATCH_SIZE", 100)
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
		AutoMigrate: false, // db migration is handled by the indexer
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
		ChainId:    viper.GetString("CHAIN_ID"),
		VmType:     vmType,
		RpcUrl:     viper.GetString("RPC_URL"),
		RestUrl:    viper.GetString("REST_URL"),
		JsonRpcUrl: viper.GetString("JSON_RPC_URL"),
	}

	config := &Config{
		listenAddr:  viper.GetString("LISTEN_ADDR"),
		dbConfig:    dc,
		chainConfig: cc,
		logLevel:    viper.GetString("LOG_LEVEL"),
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

func (c Config) GetListenAddr() string {
	return c.listenAddr
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

func (c Config) Validate() error {
	if len(c.listenAddr) == 0 {
		return fmt.Errorf("PORT is required")
	}
	if err := c.dbConfig.Validate(); err != nil {
		return err
	}
	if err := c.chainConfig.Validate(); err != nil {
		return err
	}
	return nil
}

func (cc ChainConfig) Validate() error {
	if len(cc.ChainId) == 0 {
		return fmt.Errorf("CHAIN_ID is required")
	}
	if len(cc.RpcUrl) == 0 {
		return fmt.Errorf("RPC_URL is required")
	}
	if _, err := url.Parse(cc.RpcUrl); err != nil {
		return fmt.Errorf("RPC_URL(%s) is invalid: %s", cc.RpcUrl, err)
	}
	if len(cc.RestUrl) == 0 {
		return fmt.Errorf("REST_URL is required")
	}
	if _, err := url.Parse(cc.RestUrl); err != nil {
		return fmt.Errorf("REST_URL(%s) is invalid: %s", cc.RestUrl, err)
	}
	if cc.VmType == types.EVM {
		if len(cc.JsonRpcUrl) == 0 {
			return fmt.Errorf("JSON_RPC_URL is required")
		}
		if _, err := url.Parse(cc.JsonRpcUrl); err != nil {
			return fmt.Errorf("JSON_RPC_URL(%s) is invalid: %s", cc.JsonRpcUrl, err)
		}
	}
	return nil
}
