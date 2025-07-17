package main

import "github.com/initia-labs/rollytics/cmd"

var (
	Version    = "dev"
	CommitHash = "unknown"
)

func main() {
	cmd.SetVersion(Version, CommitHash)
	if err := cmd.NewRootCmd().Execute(); err != nil {
		panic(err)
	}
}
