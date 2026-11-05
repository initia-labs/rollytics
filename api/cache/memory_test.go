package cache

import (
	"errors"
	"fmt"
	"testing"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/stretchr/testify/require"
)

func TestResponseBodyBudget(t *testing.T) {
	const mib = uint64(1024 * 1024)
	for _, tc := range []struct {
		name       string
		limit      uint64
		cgroupErr  error
		host       uint64
		hostErr    error
		want       uint64
		source     string
		wantSystem bool
	}{
		{name: "512 MiB pod", limit: 512 * mib, want: 16 * mib, source: "cgroup"},
		{name: "1 GiB pod ignores large host", limit: 1024 * mib, host: 65536 * mib, want: 32 * mib, source: "cgroup"},
		{name: "2 GiB pod", limit: 2048 * mib, want: 64 * mib, source: "cgroup"},
		{name: "large pod capped", limit: 65536 * mib, want: 256 * mib, source: "cgroup"},
		{name: "unlimited cgroup", cgroupErr: memlimit.ErrNoLimit, host: 4096 * mib, want: 128 * mib, source: "system", wantSystem: true},
		{name: "outside cgroup", cgroupErr: memlimit.ErrNoCgroup, host: 1024 * mib, want: 32 * mib, source: "system", wantSystem: true},
		{name: "unsupported platform", cgroupErr: fmt.Errorf("wrapped: %w", memlimit.ErrCgroupsNotSupported), host: 2048 * mib, want: 64 * mib, source: "system", wantSystem: true},
		{name: "cgroup read failure", cgroupErr: errors.New("permission denied"), host: 65536 * mib, want: 32 * mib, source: "fallback"},
		{name: "system failure", cgroupErr: memlimit.ErrNoLimit, hostErr: errors.New("unavailable"), want: 32 * mib, source: "fallback", wantSystem: true},
		{name: "zero memory", want: 32 * mib, source: "fallback"},
		{name: "tiny memory never yields unlimited cache", limit: 1, want: 4, source: "cgroup"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			budget, source := responseBodyBudget(func() (uint64, error) { return tc.limit, tc.cgroupErr }, func() (uint64, error) {
				called = true
				return tc.host, tc.hostErr
			})
			require.Equal(t, tc.want, budget)
			require.Equal(t, tc.source, source)
			require.Equal(t, tc.wantSystem, called)
			require.Positive(t, budget/4)
		})
	}
}
