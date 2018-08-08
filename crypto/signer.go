package crypto

import (
	"crypto/elliptic"
	"crypto/sha256"
	"dgateway/log"
	"encoding/hex"

	"github.com/btcsuite/btcd/btcec"
	"github.com/spf13/viper"
)

var cryptoLogger = log.New(viper.GetString("loglevel"), "crypto")

// Signer 保存公钥私钥对
type Signer struct {
	privKey *btcec.PrivateKey
}

// Sign 对digest做签名
func (signer *Signer) Sign(digest []byte) ([]byte, error) {
	hasher := sha256.New()
	hasher.Write(digest)
	digestHash := hasher.Sum(nil)
	sig, err := signer.privKey.Sign(digestHash)
	if err != nil {
		return nil, err
	}
	return sig.Serialize(), nil
}

func TextToPub(pub string) ([]byte, error) {
	b, err := hex.DecodeString(pub)
	if err != nil {
		return b, err
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), b)
	return append(x.Bytes(), y.Bytes()...), nil
}

func SignerFromText(text string) *Signer {
	privBytes, err := hex.DecodeString(text)
	if err != nil {
		panic("decode priv key failed")
	}
	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privBytes)
	return &Signer{privKey}
}
