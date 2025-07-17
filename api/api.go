package api

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"

	"github.com/initia-labs/rollytics/api/docs"
	"github.com/initia-labs/rollytics/api/handler"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
)

type Api struct {
	app    *fiber.App
	cfg    *config.Config
	logger *slog.Logger
	db     *orm.Database
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *Api {
	app := fiber.New(fiber.Config{
		AppName:               "Rollytics API",
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			errString := err.Error()
			if !strings.HasPrefix(errString, "Cannot GET") {
				logger.Error(errString, "path", c.Path(), "method", c.Method())
			}

			code := fiber.StatusInternalServerError
			e := &fiber.Error{}
			if errors.As(err, &e) {
				code = e.Code
			}

			return c.Status(code).SendString(errString)
		},
	})

	app.Get("/health", health)
	api := app.Group("/indexer")
	handler.Register(api, db, cfg, logger)

	// Swagger documentation
	swaggerConfig := swagger.Config{
		URL:         "/swagger/doc.json",
		DeepLinking: true,
		TagsSorter: template.JS(`function(a, b) {
			const order = ["Block", "Tx", "EVM Tx", "NFT"];
			return order.indexOf(a) - order.indexOf(b);
		}`),
	}

	app.Get("/swagger/*", swagger.New(swaggerConfig))

	return &Api{
		app:    app,
		cfg:    cfg,
		logger: logger,
		db:     db,
	}
}

// @title Rollytics API
// @version 1.0
// @description Rollytics API documentation
// @BasePath /indexer

// @tag.name Block
// @tag.description Block related operations

// @tag.name Tx
// @tag.description Transaction related operations

// @tag.name EVM Tx
// @tag.description EVM transaction related operations

// @tag.name NFT
// @tag.description NFT related operations
func (a *Api) Start() error {
	port := a.cfg.GetListenPort()
	docs.SwaggerInfo.Title = "Rollytics API"
	docs.SwaggerInfo.Description = "Rollytics API"

	a.logger.Info("starting API server", slog.String("addr", fmt.Sprintf("http://localhost:%s", port)))
	return a.app.Listen(":" + port)
}

func (a *Api) Shutdown() error {
	return a.app.Shutdown()
}

// health handles GET /health
// @Summary Health check
// @Tags App
// @Success 200 "OK"
// @Router /health [get]
func health(c *fiber.Ctx) error {
	return c.SendString("OK")
}
