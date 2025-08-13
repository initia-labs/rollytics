package api

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"

	"github.com/initia-labs/rollytics/api/docs"
	"github.com/initia-labs/rollytics/api/handler"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/metrics"
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

	// Add panic recovery middleware
	app.Use(func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				metrics.TrackPanic("api")
				metrics.TrackError("api", "panic")
				logger.Error("panic recovered in API handler", "path", c.Path(), "panic", r)
				_ = c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
			}
		}()
		return c.Next()
	})

	// Add metrics middleware
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()

		// Track requests in flight
		metrics.GetMetrics().HTTP.RequestsInFlight.Inc()
		defer metrics.GetMetrics().HTTP.RequestsInFlight.Dec()

		// Continue with request
		err := c.Next()

		// Track metrics after request completion
		duration := time.Since(start).Seconds()
		method := c.Method()
		path := c.Route().Path
		if path == "" {
			path = c.Path()
		}

		// Get handler pattern and status class
		handler := metrics.GetHandlerPattern(path)
		statusCode := c.Response().StatusCode()
		statusClass := metrics.GetStatusClass(statusCode)

		// Track HTTP metrics
		metrics.GetMetrics().HTTP.RequestsTotal.WithLabelValues(method, handler, statusClass).Inc()
		metrics.GetMetrics().HTTP.RequestDuration.WithLabelValues(method, handler).Observe(duration)

		// Track detailed metrics for slow or important requests
		if metrics.ShouldTrackDetailed(duration, path) {
			bucket := metrics.GetDurationBucket(duration)
			if bucket != "" {
				metrics.GetMetrics().HTTP.SlowRequests.WithLabelValues(method, path, bucket).Inc()
			}
		}

		// Track endpoint performance for top endpoints analysis
		metrics.TrackEndpoint(path, duration)

		// Track HTTP errors
		if err != nil {
			errorType := "server_error"
			fiberErr := &fiber.Error{}
			if errors.As(err, &fiberErr) {
				switch {
				case fiberErr.Code == 401:
					errorType = "unauthorized"
				case fiberErr.Code == 403:
					errorType = "forbidden"
				case fiberErr.Code == 404:
					errorType = "not_found"
				case fiberErr.Code >= 400 && fiberErr.Code < 500:
					errorType = "client_error"
				case fiberErr.Code >= 500:
					errorType = "server_error"
				}
			}
			metrics.GetMetrics().HTTP.ErrorsTotal.WithLabelValues(handler, errorType).Inc()
		}

		return err
	})

	handler.Register(app, db, cfg, logger)

	// Swagger documentation
	swaggerConfig := swagger.Config{
		URL:         "/swagger/doc.json",
		DeepLinking: true,
		TagsSorter: template.JS(`function(a, b) {
			const order = ["Block", "Tx", "EVM Tx", "EVM Internal Tx", "NFT"];
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

// @tag.name EVM Internal Tx
// @tag.description EVM internal transaction related operations

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
