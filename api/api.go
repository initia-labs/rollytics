package api

import (
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/swagger"

	"github.com/initia-labs/rollytics/api/docs"
	"github.com/initia-labs/rollytics/api/handler"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/metrics"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
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
		ErrorHandler:          createErrorHandler(logger),
	})

	addCORS(app, cfg)
	addPanicRecoveryMiddleware(app, logger)
	addMetricsMiddleware(app)
	handler.Register(app, db, cfg, logger)
	setupSwagger(app, cfg)

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

func normalizeOrigins(in []string) []string {
	out := make([]string, 0, len(in))
	for _, o := range in {
		o = strings.ToLower(strings.TrimSpace(o))
		if o != "" {
			out = append(out, o)
		}
	}
	return out
}

func containsWildcard(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return true
		}
	}
	return false
}

func makeAllowOriginsFunc(allowed []string) func(string) bool {
	return func(origin string) bool {
		orig := strings.ToLower(strings.TrimSpace(origin))
		if orig == "" {
			// No Origin header -> allow request to pass (no CORS header needed)
			return true
		}
		for _, pat := range allowed {
			if pat == orig {
				return true
			}
			if strings.HasPrefix(pat, "*.") {
				domain := strings.TrimPrefix(pat, "*.")
				if strings.HasSuffix(orig, "."+domain) {
					return true
				}
			}
		}
		return false
	}
}

// addCORS configures Cross-Origin Resource Sharing (CORS) for the API.
func addCORS(app *fiber.App, cfg *config.Config) {
	corsCfg := cfg.GetCORSConfig()
	if corsCfg == nil || !corsCfg.Enabled {
		return
	}

	allowedOrigins := normalizeOrigins(corsCfg.AllowOrigin)

	mwCfg := cors.Config{
		AllowMethods:     strings.Join(corsCfg.AllowMethods, ","),
		AllowHeaders:     strings.Join(corsCfg.AllowHeaders, ","),
		ExposeHeaders:    strings.Join(corsCfg.ExposeHeaders, ","),
		AllowCredentials: corsCfg.AllowCredentials,
		MaxAge:           corsCfg.MaxAge,
	}

	if containsWildcard(allowedOrigins) {
		if corsCfg.AllowCredentials {
			// With credentials, browsers don't accept '*' in ACAO, so use function to allow all origins.
			mwCfg.AllowOriginsFunc = func(string) bool { return true }
		} else {
			// No credentials -> can use '*'
			mwCfg.AllowOrigins = "*"
		}
	} else {
		mwCfg.AllowOriginsFunc = makeAllowOriginsFunc(allowedOrigins)
	}

	app.Use(cors.New(mwCfg))
}

// addPanicRecoveryMiddleware adds panic recovery middleware to the app
func addPanicRecoveryMiddleware(app *fiber.App, logger *slog.Logger) {
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
}

// addMetricsMiddleware adds metrics tracking middleware to the app
func addMetricsMiddleware(app *fiber.App) {
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		httpMetrics := metrics.GetMetrics().HTTPMetrics()

		httpMetrics.RequestsInFlight.Inc()
		defer httpMetrics.RequestsInFlight.Dec()

		defer handlePanicMetrics(start, c, httpMetrics)

		err := c.Next()
		trackRequestMetrics(start, c, httpMetrics, err)
		return err
	})
}

// handlePanicMetrics handles metrics tracking during panic recovery
func handlePanicMetrics(start time.Time, c *fiber.Ctx, httpMetrics *metrics.HTTPMetrics) {
	if r := recover(); r != nil {
		duration := time.Since(start).Seconds()
		method := c.Method()
		path := getRequestPath(c)
		handler := metrics.GetHandlerPattern(path)

		httpMetrics.RequestsTotal.WithLabelValues(method, handler, "5xx").Inc()
		httpMetrics.RequestDuration.WithLabelValues(method, handler).Observe(duration)
		httpMetrics.ErrorsTotal.WithLabelValues(handler, "server_error").Inc()
		metrics.TrackEndpoint(path, duration)

		panic(r)
	}
}

// trackRequestMetrics tracks metrics for completed requests
func trackRequestMetrics(start time.Time, c *fiber.Ctx, httpMetrics *metrics.HTTPMetrics, err error) {
	duration := time.Since(start).Seconds()
	method := c.Method()
	path := getRequestPath(c)
	handler := metrics.GetHandlerPattern(path)
	statusCode := c.Response().StatusCode()
	statusClass := metrics.GetStatusClass(statusCode)

	httpMetrics.RequestsTotal.WithLabelValues(method, handler, statusClass).Inc()
	httpMetrics.RequestDuration.WithLabelValues(method, handler).Observe(duration)

	trackSlowRequests(duration, path, method, httpMetrics)
	metrics.TrackEndpoint(path, duration)
	trackHTTPErrors(err, handler, httpMetrics)
}

// getRequestPath gets the request path for metrics tracking
func getRequestPath(c *fiber.Ctx) string {
	path := c.Route().Path
	if path == "" {
		path = c.Path()
	}
	return path
}

// trackSlowRequests tracks detailed metrics for slow requests
func trackSlowRequests(duration float64, path, method string, httpMetrics *metrics.HTTPMetrics) {
	if metrics.ShouldTrackDetailed(duration, path) {
		bucket := metrics.GetDurationBucket(duration)
		if bucket != "" {
			httpMetrics.SlowRequests.WithLabelValues(method, path, bucket).Inc()
		}
	}
}

// trackHTTPErrors tracks HTTP error metrics
func trackHTTPErrors(err error, handler string, httpMetrics *metrics.HTTPMetrics) {
	if err != nil {
		errorType := determineErrorType(err)
		httpMetrics.ErrorsTotal.WithLabelValues(handler, errorType).Inc()
	}
}

// determineErrorType determines the error type for metrics tracking
func determineErrorType(err error) string {
	fiberErr := &fiber.Error{}
	if errors.As(err, &fiberErr) {
		switch {
		case fiberErr.Code == 401:
			return "unauthorized"
		case fiberErr.Code == 403:
			return "forbidden"
		case fiberErr.Code == 404:
			return "not_found"
		case fiberErr.Code >= 400 && fiberErr.Code < 500:
			return "client_error"
		case fiberErr.Code >= 500:
			return "server_error"
		}
	}
	return "server_error"
}

// setupSwagger configures swagger documentation
func setupSwagger(app *fiber.App, cfg *config.Config) {
	swaggerConfig := swagger.Config{
		URL:         "/swagger/doc.json",
		DeepLinking: true,
		TagsSorter: template.JS(`function(a, b) {
			const order = ["Block", "Tx", "EVM Tx", "EVM Internal Tx", "NFT"];
			return order.indexOf(a) - order.indexOf(b);
		}`),
	}

	if cfg.GetVmType() != types.EVM {
		setupNonEVMSwagger(app)
	}

	app.Get("/swagger/*", swagger.New(swaggerConfig))
}

// setupNonEVMSwagger sets up swagger for non-EVM configurations
func setupNonEVMSwagger(app *fiber.App) {
	app.Get("/swagger/doc.json", func(c *fiber.Ctx) error {
		swaggerData := docs.SwaggerInfo.ReadDoc()

		var spec map[string]any
		if err := json.Unmarshal([]byte(swaggerData), &spec); err != nil {
			return c.Type("json").SendString(swaggerData)
		}

		if paths, ok := spec["paths"].(map[string]any); ok {
			for path := range paths {
				if strings.Contains(path, "/evm-") {
					delete(paths, path)
				}
			}
		}

		return c.JSON(spec)
	})
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

	listenAddr := ":" + port

	a.logger.Info("starting API server", slog.String("addr", listenAddr))
	return a.app.Listen(listenAddr)
}

func (a *Api) Shutdown() error {
	return a.app.Shutdown()
}
