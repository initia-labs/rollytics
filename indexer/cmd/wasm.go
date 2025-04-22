//go:build wasm
// +build wasm

package cmd

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/initia-labs/miniwasm/app"
)

var txConfig client.TxConfig

func init() {
	txConfig = app.MakeEncodingConfig().TxConfig
}
