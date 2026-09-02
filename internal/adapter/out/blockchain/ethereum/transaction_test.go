package ethereum

import (
	"encoding/hex"
	"testing"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	"github.com/rajabinekoo/sigryx/pkg/hdwallet"
)

func TestLegacyTransactionMatchesEIP155Vector(t *testing.T) {
	privateKey, _ := hex.DecodeString("4646464646464646464646464646464646464646464646464646464646464646")
	result, err := signTransaction(privateKey, domain.EthereumTransaction{
		Type:     domain.TransactionTypeLegacy,
		ChainID:  1,
		Nonce:    9,
		GasLimit: 21000,
		GasPrice: "20000000000",
		To:       "0x3535353535353535353535353535353535353535",
		Value:    "1000000000000000000",
	})
	if err != nil {
		t.Fatal(err)
	}

	const expected = "f86c098504a817c800825208943535353535353535353535353535353535353535880de0b6b3a76400008025a028ef61340bd939bc2195fe537567866003e1a15d3c71ff63e1590620aa636276a067cbe9d8997f761aecb703304b3800ccf555c9f3dc64214b297fb1966a3b6d83"
	if got := hex.EncodeToString(result.RawTransaction); got != expected {
		t.Fatalf("signed tx = %s\nwant      = %s", got, expected)
	}

	publicKey, err := hdwallet.UncompressedPublicKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := verifyTransaction(publicKey, result.RawTransaction)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("official EIP-155 transaction did not verify")
	}
}

func TestEIP1559TransactionSignAndVerify(t *testing.T) {
	privateKey := make([]byte, 32)
	privateKey[31] = 1
	publicKey, err := hdwallet.UncompressedPublicKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}

	result, err := signTransaction(privateKey, domain.EthereumTransaction{
		Type:                 domain.TransactionTypeEIP1559,
		ChainID:              1,
		Nonce:                3,
		GasLimit:             65000,
		MaxPriorityFeePerGas: "1500000000",
		MaxFeePerGas:         "30000000000",
		To:                   "0xdAC17F958D2ee523a2206206994597C13D831ec7",
		Value:                "0",
		Data:                 "0xa9059cbb000000000000000000000000353535353535353535353535353535353535353500000000000000000000000000000000000000000000000000000000000f4240",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RawTransaction) == 0 || result.RawTransaction[0] != 0x02 {
		t.Fatal("expected EIP-1559 typed transaction")
	}
	valid, err := verifyTransaction(publicKey, result.RawTransaction)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("EIP-1559 transaction did not verify")
	}

	otherPrivateKey := make([]byte, 32)
	otherPrivateKey[31] = 2
	otherPublicKey, _ := hdwallet.UncompressedPublicKey(otherPrivateKey)
	valid, err = verifyTransaction(otherPublicKey, result.RawTransaction)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("transaction verified with another wallet")
	}
}
