package cache_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/initia-labs/rollytics/api/cache"
)

func TestCacheExpiration(t *testing.T) {
	testCases := []struct {
		name            string
		expiration      time.Duration
		waitBeforeHit   time.Duration
		waitAfterExpire time.Duration
		expectFail      bool
	}{
		{
			name:            "Sub-second expiration",
			expiration:      250 * time.Millisecond,
			waitBeforeHit:   100 * time.Millisecond,
			waitAfterExpire: 300 * time.Millisecond, // Wait 300ms, which is > 250ms
			expectFail:      true,                   // We expect this test to fail, proving the bug
		},
		{
			name:            "Supra-second expiration",
			expiration:      1 * time.Second,
			waitBeforeHit:   200 * time.Millisecond,
			waitAfterExpire: 1100 * time.Millisecond, // Wait 1.1s, which is > 1s
			expectFail:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/test", cache.WithExpiration(tc.expiration), func(c *fiber.Ctx) error {
				return c.SendString(time.Now().String())
			})

			// 1. First request - should be a cache miss
			req1, err := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
			require.NoError(t, err)
			resp1, err := app.Test(req1, -1) // -1 disables timeout
			require.NoError(t, err)
			require.Equal(t, "miss", resp1.Header.Get("X-Cache"))
			body1, _ := io.ReadAll(resp1.Body)
			_ = resp1.Body.Close()

			// 2. Wait for a duration shorter than expiration and make a second request
			time.Sleep(tc.waitBeforeHit)
			req2, err := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
			require.NoError(t, err)
			resp2, err := app.Test(req2, -1)
			require.NoError(t, err)
			require.Equal(t, "hit", resp2.Header.Get("X-Cache"))
			body2, _ := io.ReadAll(resp2.Body)
			_ = resp2.Body.Close()
			require.Equal(t, body1, body2, "Content should be the same for a cache hit")

			// 3. Wait for a duration longer than expiration and make a third request
			time.Sleep(tc.waitAfterExpire)
			req3, err := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
			require.NoError(t, err)
			resp3, err := app.Test(req3, -1)
			require.NoError(t, err)

			// This is the crucial assertion
			finalCacheStatus := resp3.Header.Get("X-Cache")
			body3, _ := io.ReadAll(resp3.Body)
			_ = resp3.Body.Close()

			if tc.expectFail {
				// For the sub-second test, we expect a 'hit' here, proving the cache did NOT expire
				require.Equal(t, "hit", finalCacheStatus, "CACHE DID NOT EXPIRE AS EXPECTED - BUG CONFIRMED")
				t.Logf("Test confirmed the bug: cache was hit after %s, despite %s expiration", tc.waitAfterExpire, tc.expiration)
			} else {
				// For the 1s+ test, we expect a 'miss', proving the cache expired correctly
				require.Equal(t, "miss", finalCacheStatus, "Cache should have expired")
				require.NotEqual(t, body1, body3, "Content should be different after cache expiration")
				t.Log("Test passed: cache expired as expected.")
			}
		})
	}
}

func TestCacheMaxBytes(t *testing.T) {
	app := fiber.New()
	calls := 0
	app.Get("/test", cache.WithExpirationAndMaxBytes(time.Minute, 8), func(c *fiber.Ctx) error {
		calls++
		return c.SendString(c.Query("body"))
	})

	request := func(body, status string, expectedCalls int) {
		t.Helper()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/test?body="+body, nil)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		got, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, body, string(got))
		require.Equal(t, status, resp.Header.Get("X-Cache"))
		require.Equal(t, expectedCalls, calls)
	}

	// A body exactly at the limit is cached.
	request("12345678", "miss", 1)
	request("12345678", "hit", 1)
	// Oversized responses are delivered intact without being retained.
	request("123456789", "unreachable", 2)
	request("123456789", "unreachable", 3)
	request("12345678", "hit", 3)
	// A different query key must evict the previous body to fit the budget.
	request("abcdefgh", "miss", 4)
	request("abcdefgh", "hit", 4)
	request("12345678", "miss", 5)
}
