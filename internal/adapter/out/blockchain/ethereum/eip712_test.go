package ethereum

import (
	"encoding/hex"
	"testing"

	"github.com/rajabinekoo/sigryx/pkg/hdwallet"
)

const eip712Mail = `{
  "types": {
    "EIP712Domain": [
      {"name":"name","type":"string"},
      {"name":"version","type":"string"},
      {"name":"chainId","type":"uint256"},
      {"name":"verifyingContract","type":"address"}
    ],
    "Person": [
      {"name":"name","type":"string"},
      {"name":"wallet","type":"address"}
    ],
    "Mail": [
      {"name":"from","type":"Person"},
      {"name":"to","type":"Person"},
      {"name":"contents","type":"string"}
    ]
  },
  "primaryType":"Mail",
  "domain": {
    "name":"Ether Mail",
    "version":"1",
    "chainId":1,
    "verifyingContract":"0xCcCCccccCCCCcCCCCCCcCcCccCcCCCcCcccccccC"
  },
  "message": {
    "from":{"name":"Cow","wallet":"0xCD2a3d9F938E13CD947Ec05AbC7FE734Df8DD826"},
    "to":{"name":"Bob","wallet":"0xbBbBBBBbbBBBbbbBbbBbbbbBBbBbbbbBbBbbBBbB"},
    "contents":"Hello, Bob!"
  }
}`

func TestTypedDataDigestMatchesEIP712MailVector(t *testing.T) {
	digest, err := typedDataDigest([]byte(eip712Mail))
	if err != nil {
		t.Fatal(err)
	}
	const expected = "be609aee343fb3c4b28e1df9e632fca64fcfaede20f02e86244efddf30957bd2"
	if got := hex.EncodeToString(digest[:]); got != expected {
		t.Fatalf("digest = %s, want %s", got, expected)
	}
}

func TestTypedDataSignatureMatchesOfficialEIP712Example(t *testing.T) {
	privateKey, _ := hex.DecodeString("c85ef7d79691fe79573b1a7064c19c1a9819ebdbd1faaab1a8ec92344438aaf4")
	signature, _, err := New().SignTypedData(privateKey, []byte(eip712Mail))
	if err != nil {
		t.Fatal(err)
	}

	const expected = "4355c47d63924e8a72e509b65029052eb6c299d53a04e167c5775fd466751c9d07299936d304c153f6443dfa05f40ff007d72911b6f72307f996231605b915621c"
	if got := hex.EncodeToString(signature); got != expected {
		t.Fatalf("signature = %s, want %s", got, expected)
	}
}

func TestTypedDataSignAndVerify(t *testing.T) {
	privateKey := make([]byte, 32)
	privateKey[31] = 1
	adapter := New()
	publicKey, err := hdPublicKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}

	signature, _, err := adapter.SignTypedData(privateKey, []byte(eip712Mail))
	if err != nil {
		t.Fatal(err)
	}
	valid, _, err := adapter.VerifyTypedData(publicKey, []byte(eip712Mail), signature)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("typed data signature did not verify")
	}

	tampered := append([]byte(nil), signature...)
	if tampered[64] == 27 {
		tampered[64] = 28
	} else {
		tampered[64] = 27
	}
	valid, _, err = adapter.VerifyTypedData(publicKey, []byte(eip712Mail), tampered)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("typed data signature verified with wrong recovery id")
	}
}

func hdPublicKey(privateKey []byte) ([]byte, error) {
	return hdwallet.UncompressedPublicKey(privateKey)
}
