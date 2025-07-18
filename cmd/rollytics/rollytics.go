package main

import (
	"github.com/initia-labs/rollytics/cmd"
	"github.com/initia-labs/rollytics/config"
)

var (
	Version    = "dev"
	CommitHash = "unknown"
)

func main() {
	config.SetBuildInfo(Version, CommitHash)
	if err := cmd.NewRootCmd().Execute(); err != nil {
		panic(err)
	}
}
