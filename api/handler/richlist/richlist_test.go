package richlist

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

func TestParseDenomFromPath(t *testing.T) {
	app := fiber.New()
	app.Get("/indexer/richlist/v1/*", func(c *fiber.Ctx) error {
		denom, err := parseDenomFromPath(c)
		if err != nil {
			return err
		}
		return c.SendString(denom)
	})

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "plain", path: "/indexer/richlist/v1/iusd-lp", want: "iusd-lp"},
		{name: "space encoded", path: "/indexer/richlist/v1/cbl%20iusd-slp", want: "cbl iusd-slp"},
		{name: "slash encoded", path: "/indexer/richlist/v1/move%2Fasdasd", want: "move/asdasd"},
		{name: "slash raw", path: "/indexer/richlist/v1/move/asdasd", want: "move/asdasd"},
		{name: "nested slash raw", path: "/indexer/richlist/v1/move/1/abc", want: "move/1/abc"},
		{name: "ibc raw", path: "/indexer/richlist/v1/ibc/37a3fb4fed4ca04ed6d9e5da36c6d27248645f0e22f585576a1488b8a89c5a50", want: "ibc/37a3fb4fed4ca04ed6d9e5da36c6d27248645f0e22f585576a1488b8a89c5a50"},
		{name: "uppercase", path: "/indexer/richlist/v1/CBL%20iUSD-slp", want: "cbl iusd-slp"},
		{name: "empty", path: "/indexer/richlist/v1/", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(fiber.MethodGet, tt.path, nil)
			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			if tt.wantErr {
				require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
				return
			}
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
			require.Equal(t, tt.want, string(body))
		})
	}
}
