package cache

import (
	"errors"
	"log/slog"

	"github.com/KimMachineGun/automemlimit/memlimit"
)

const (
	fallbackBodyBudget = uint64(32 * 1024 * 1024)
	maxBodyBudget      = uint64(256 * 1024 * 1024)
)

// ResponseBodyBudget computes a startup budget for a group of response caches:
// 1/32 of detected memory, capped at 256 MiB. Call once and divide across routes.
// It excludes cache metadata and in-flight allocations and does not change GOMEMLIMIT.
func ResponseBodyBudget() uint64 {
	budget, source := responseBodyBudget(memlimit.FromCgroup, memlimit.FromSystem)
	slog.Info("response cache memory budget", "bytes", budget, "source", source)
	return budget
}

func responseBodyBudget(cgroup, system func() (uint64, error)) (uint64, string) {
	limit, err := cgroup()
	source := "cgroup"
	if errors.Is(err, memlimit.ErrNoLimit) || errors.Is(err, memlimit.ErrNoCgroup) || errors.Is(err, memlimit.ErrCgroupsNotSupported) {
		limit, err = system()
		source = "system"
	}
	// Unexpected cgroup errors must not silently select the host's larger RAM.
	if err != nil || limit == 0 {
		return fallbackBodyBudget, "fallback"
	}
	return min(max(limit/32, 4), maxBodyBudget), source
}
