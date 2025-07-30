package types

// SeqInfoName represents the name of a sequence info entry
type SeqInfoName string

const (
	SeqInfoTx            SeqInfoName = "tx"
	SeqInfoEvmTx         SeqInfoName = "evm_tx"
	SeqInfoEvmInternalTx SeqInfoName = "evm_internal_tx"
)
