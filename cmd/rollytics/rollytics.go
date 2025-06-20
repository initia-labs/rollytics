package main

import "github.com/initia-labs/rollytics/cmd"

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		panic(err)
	}
}
