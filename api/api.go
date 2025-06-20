package api

import (
	"fmt"
	"html/template"
	"log/slog"

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

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *Api {
	return &Api{
		cfg:    cfg,
		logger: logger,
		db:     db,
	}
}

// @title Rollytics API
// @version 1.0
// @description Rollytics API documentation
// @BasePath /indexer

// @tag.name Blocks
// @tag.description Block related operations

// @tag.name Transactions
// @tag.description Transaction related operations

// @tag.name NFT
// @tag.description NFT related operations

// @tag.name Evm Transactions
// @tag.description EVM transaction related operations
func (a *Api) Start() error {
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
		TagsSorter: template.JS(`function(a, b) {
			const order = ["Blocks", "Transactions", "NFT", "Evm Transactions"];
			return order.indexOf(a) - order.indexOf(b);
		}`),
	}

	app.Get("/swagger/*", swagger.New(swaggerConfig))

	port := a.cfg.GetListenPort()

	docs.SwaggerInfo.Host = fmt.Sprintf("localhost:%s", port)
	docs.SwaggerInfo.Title = "Rollytics API"
	docs.SwaggerInfo.Description = "Rollytics API"

	a.logger.Info("starting API server", slog.String("addr", fmt.Sprintf("http://localhost:%s", port)))

	return app.Listen(":" + port)
}
