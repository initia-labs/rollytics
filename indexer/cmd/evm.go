//go:build evm
// +build evm

package cmd

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/initia-labs/minievm/app"
)

var txConfig client.TxConfig

func init() {
	txConfig = app.MakeEncodingConfig().TxConfig
}
