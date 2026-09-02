package domain

type TransactionType string

const (
	TransactionTypeLegacy  TransactionType = "LEGACY"
	TransactionTypeEIP1559 TransactionType = "EIP1559"
)

type AccessListEntry struct {
	Address     string
	StorageKeys []string
}

type EthereumTransaction struct {
	Type                 TransactionType
	ChainID              uint64
	Nonce                uint64
	GasLimit             uint64
	GasPrice             string
	MaxPriorityFeePerGas string
	MaxFeePerGas         string
	To                   string
	Value                string
	Data                 string
	AccessList           []AccessListEntry
}

type DataFormat string

const (
	DataFormatRaw  DataFormat = "RAW"
	DataFormatJSON DataFormat = "JSON"
)
