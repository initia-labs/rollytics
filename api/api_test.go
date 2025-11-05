package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
)

func newTestAppWithCORS(corsCfg *config.CORSConfig) *fiber.App {
	app := fiber.New()
	cfg := &config.Config{}
	cfg.SetCORSConfig(corsCfg)
	addCORS(app, cfg)
	// simple route for testing
	app.Get("/ping", func(c *fiber.Ctx) error { return c.SendString("pong") })
	return app
}

func TestCORS_WildcardOrigin(t *testing.T) {
	app := newTestAppWithCORS(&config.CORSConfig{
		Enabled:      true,
		AllowOrigin:  []string{"*"},
		AllowMethods: []string{"GET", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization"},
	})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://moro.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://moro.example" && got != "*" {
		// fiber sets ACAO to echo origin when credentials true; allow either echo or '*'
		t.Fatalf("expected ACAO to be echo origin or '*', got %q", got)
	}
}

func TestCORS_ExactOriginAllowed(t *testing.T) {
	app := newTestAppWithCORS(&config.CORSConfig{
		Enabled:      true,
		AllowOrigin:  []string{"https://example.com"},
		AllowMethods: []string{"GET"},
		AllowHeaders: []string{"Content-Type"},
	})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://example.com")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("expected ACAO to be https://example.com, got %q", got)
	}
}

func TestCORS_SubdomainPattern(t *testing.T) {
	app := newTestAppWithCORS(&config.CORSConfig{
		Enabled:      true,
		AllowOrigin:  []string{"*.initia.xyz"},
		AllowMethods: []string{"GET"},
		AllowHeaders: []string{"Content-Type"},
	})

	// subdomain should be allowed
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://app.initia.xyz")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://app.initia.xyz" {
		t.Fatalf("expected ACAO to echo subdomain origin, got %q", got)
	}

	// bare domain should NOT be allowed when only a subdomain pattern is provided
	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req2.Header.Set("Origin", "https://initia.xyz")
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp2.Body.Close()

	if got := resp2.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no ACAO header for bare domain when only subdomain pattern allowed, got %q", got)
	}
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	app := newTestAppWithCORS(&config.CORSConfig{
		Enabled:      true,
		AllowOrigin:  []string{"https://doi.com"},
		AllowMethods: []string{"GET"},
		AllowHeaders: []string{"Content-Type"},
	})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://moro.com")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no ACAO header for disallowed origin, got %q", got)
	}
}

func TestCORS_EmptyOriginAllowed(t *testing.T) {
	app := newTestAppWithCORS(&config.CORSConfig{
		Enabled:      true,
		AllowOrigin:  []string{"https://example.com"},
		AllowMethods: []string{"GET"},
		AllowHeaders: []string{"Content-Type"},
	})

	// No Origin header simulates non-browser or same-origin request; should succeed without CORS headers
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no ACAO header when Origin is absent, got %q", got)
	}
}

func TestCORS_PreflightOPTIONS(t *testing.T) {
	app := newTestAppWithCORS(&config.CORSConfig{
		Enabled:      true,
		AllowOrigin:  []string{"https://example.com"},
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:       600,
	})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodOptions, "/ping", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("preflight failed: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		t.Fatalf("expected successful preflight status, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got == "" {
		t.Fatalf("expected ACAO header on preflight, got empty")
	}
	if got := resp.Header.Get("Access-Control-Max-Age"); got != "600" {
		t.Fatalf("expected Access-Control-Max-Age=600, got %q", got)
	}
}

func TestCORS_MaxAgeHeader_Custom(t *testing.T) {
	app := newTestAppWithCORS(&config.CORSConfig{
		Enabled:          true,
		AllowOrigin:      []string{"https://example.com"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           1234,
	})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodOptions, "/ping", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("preflight failed: %v", err)
	}
	_ = resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Max-Age"); got != "1234" {
		t.Fatalf("expected Access-Control-Max-Age=1234, got %q", got)
	}
}

// Additional CORS tests covering defaults and extra scenarios

// Defaults: Enabled=false should result in no CORS headers being added.
func TestCORS_Defaults_Disabled_NoHeaders(t *testing.T) {
	app := newTestAppWithCORS(&config.CORSConfig{
		Enabled:      false, // default per config
		AllowOrigin:  []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		MaxAge:       0,
	})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://any.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no ACAO when CORS disabled, got %q", got)
	}
}

// Defaults when enabled: origins "*", allow-credentials false => ACAO should be '*'.
func TestCORS_Defaults_Enabled_AllOrigins_NoCredentials(t *testing.T) {
	app := newTestAppWithCORS(&config.CORSConfig{
		Enabled:          true,
		AllowOrigin:      []string{"*"},                                                                     // default
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"},              // default
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"}, // default
		AllowCredentials: false,                                                                             // default
		ExposeHeaders:    []string{},                                                                        // default empty
		MaxAge:           0,                                                                                 // default
	})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://anything.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected ACAO='*' with defaults (no credentials), got %q", got)
	}
	// Max-Age default 0 => header may be absent or "0" depending on middleware; accept both
	if got := resp.Header.Get("Access-Control-Max-Age"); got != "" && got != "0" {
		t.Fatalf("expected no Access-Control-Max-Age or '0', got %q", got)
	}
}

// Wildcard with credentials=true should echo the request origin, never '*'.
func TestCORS_WildcardWithCredentialsEcho(t *testing.T) {
	app := newTestAppWithCORS(&config.CORSConfig{
		Enabled:          true,
		AllowOrigin:      []string{"*"},
		AllowMethods:     []string{"GET", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: true,
	})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	origin := "https://echo.me"
	req.Header.Set("Origin", origin)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("expected ACAO to echo origin %q with credentials=true, got %q", origin, got)
	}
}

// Mixed exact + pattern list should allow both matches and deny others.
func TestCORS_MixedExactAndPattern(t *testing.T) {
	app := newTestAppWithCORS(&config.CORSConfig{
		Enabled:      true,
		AllowOrigin:  []string{"https://doi.com", "*.initia.xyz"},
		AllowMethods: []string{"GET"},
		AllowHeaders: []string{"Content-Type"},
	})

	// exact allowed
	req1, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req1.Header.Set("Origin", "https://doi.com")
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("request1 failed: %v", err)
	}
	_ = resp1.Body.Close()

	if got := resp1.Header.Get("Access-Control-Allow-Origin"); got != "https://doi.com" {
		t.Fatalf("expected ACAO=https://doi.com, got %q", got)
	}

	// pattern subdomain allowed
	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req2.Header.Set("Origin", "https://app.initia.xyz")
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("request2 failed: %v", err)
	}
	_ = resp2.Body.Close()

	if got := resp2.Header.Get("Access-Control-Allow-Origin"); got != "https://app.initia.xyz" {
		t.Fatalf("expected ACAO to echo subdomain origin, got %q", got)
	}

	// non-matching origin denied
	req3, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req3.Header.Set("Origin", "https://not-allowed.com")
	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("request3 failed: %v", err)
	}
	_ = resp3.Body.Close()

	if got := resp3.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no ACAO for disallowed origin, got %q", got)
	}
}
