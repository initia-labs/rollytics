//go:build move
// +build move

package cmd

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/initia-labs/minimove/app"
)

var txConfig client.TxConfig

func init() {
	txConfig = app.MakeEncodingConfig().TxConfig
}
