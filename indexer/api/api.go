package api

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/api/handler"
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
		AppName:               "Rollytics Indexer API",
		DisableStartupMessage: true,
		ErrorHandler:          createErrorHandler(logger),
		ReadBufferSize:        int(cfg.GetRecvBufferSize()),
	})

	handler.Register(app, db, cfg, logger)

	return &Api{
		app:    app,
		cfg:    cfg,
		logger: logger,
		db:     db,
	}
}

// createErrorHandler creates the error handler function for the fiber app
func createErrorHandler(logger *slog.Logger) func(c *fiber.Ctx, err error) error {
	return func(c *fiber.Ctx, err error) error {
		errString := err.Error()
		if !strings.HasPrefix(errString, "Cannot GET") {
			logger.Error(errString, "path", c.Path(), "method", c.Method())
		}

		code := fiber.StatusInternalServerError
		e := &fiber.Error{}
		if errors.As(err, &e) {
			code = e.Code
		}

		if code >= fiber.StatusInternalServerError {
			return c.Status(code).SendString("Internal Server Error")
		}
		return c.Status(code).SendString(errString)
	}
}

func (a *Api) Start() error {
	port := a.cfg.GetIndexerListenPort()
	listenAddr := ":" + port

	a.logger.Info("starting API server", slog.String("addr", listenAddr), slog.Uint64("recv_buffer_size", uint64(a.cfg.GetRecvBufferSize())))
	return a.app.Listen(listenAddr)
}

func (a *Api) Shutdown() error {
	return a.app.Shutdown()
}
