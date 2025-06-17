package api

import (
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
	"github.com/initia-labs/rollytics/api/docs"
	"github.com/initia-labs/rollytics/api/handler"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
)

type Api struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *orm.Database
}

func New(cfg *config.Config, logger *slog.Logger) (*Api, error) {
	db, err := orm.OpenDB(cfg.GetDBConfig(), logger)
	if err != nil {
		return nil, err
	}

	return &Api{
		cfg:    cfg,
		logger: logger,
		db:     db,
	}, nil
}

// @title Rollytics API
// @version 1.0
// @description Rollytics API documentation
// @BasePath /indexer

// @tag.name Blocks
// @tag.description.markdown Block related operations

// @tag.name Transactions
// @tag.description.markdown Transaction related operations

// @tag.name Evm Transactions
// @tag.description.markdown EVM transaction related operations

// @tag.name Nft
// @tag.description.markdown NFT token related operations
func (a *Api) Start() {
	app := fiber.New(fiber.Config{
		AppName:               "Rollytics API",
		DisableStartupMessage: true,
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	api := app.Group("/indexer")
	handler.Register(api, a.db, a.cfg, a.logger)

	// Swagger documentation
	swaggerConfig := swagger.Config{
		URL:         "/swagger/doc.json",
		DeepLinking: true,
	}
	app.Get("/swagger/*", swagger.New(swaggerConfig))

	listenAddr := a.cfg.GetListenAddr()
	docs.SwaggerInfo.Host = listenAddr
	a.logger.Info("starting API server", slog.String("addr", listenAddr))

	if err := app.Listen(listenAddr); err != nil {
		a.logger.Error("server error", slog.Any("error", err))
		os.Exit(1)
	}
}
