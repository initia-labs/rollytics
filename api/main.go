package main

import "github.com/initia-labs/rollytics/api/cmd"

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
