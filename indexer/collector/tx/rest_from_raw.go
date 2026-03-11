package tx

import (
	"encoding/base64"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/unknownproto"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"

	"github.com/initia-labs/rollytics/types"
)

// RestTxsFromRaw builds RestTx slice from raw tx bytes (e.g. when block was recovered from DA layer).
func RestTxsFromRaw(cdc codec.Codec, rawTxs [][]byte) ([]types.RestTx, error) {
	out := make([]types.RestTx, 0, len(rawTxs))
	for _, txBytes := range rawTxs {
		var raw sdktx.TxRaw
		if err := unknownproto.RejectUnknownFieldsStrict(txBytes, &raw, cdc.InterfaceRegistry()); err != nil {
			return nil, err
		}
		if err := cdc.Unmarshal(txBytes, &raw); err != nil {
			return nil, err
		}

		var body sdktx.TxBody
		if err := cdc.Unmarshal(raw.BodyBytes, &body); err != nil {
			return nil, err
		}
		bodyJSON, err := cdc.MarshalJSON(&body)
		if err != nil {
			return nil, err
		}

		var authInfo sdktx.AuthInfo
		if err := cdc.Unmarshal(raw.AuthInfoBytes, &authInfo); err != nil {
			return nil, err
		}
		authJSON, err := cdc.MarshalJSON(&authInfo)
		if err != nil {
			return nil, err
		}

		sigs := make([]string, 0, len(raw.Signatures))
		for _, s := range raw.Signatures {
			sigs = append(sigs, base64.StdEncoding.EncodeToString(s))
		}

		out = append(out, types.RestTx{
			Body:       bodyJSON,
			AuthInfo:   authJSON,
			Signatures: sigs,
		})
	}
	return out, nil
}
