package crypto_test

import (
	"dgateway/crypto"
	"testing"
)

func TestSigner(t *testing.T) {
	signer := crypto.NewSecureSigner("04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787", "3722834BCB13F7308C28907B69A99DB462F39036")
	signer.InitKeystoreParam("C51C9CB7A7EC9D12BB37B3700856690719A44056B750AB03A21247A4903BF3CB", "0daf7126-ebbb-4b2d-86f8-a480c1fd45a8", "http://47.98.185.203:8976/key/sign")
	var testData = []byte("test data")
	sig, err := signer.Sign(testData)
	if err != nil {
		t.Errorf("sing fail:%v", err)
	}
	digest := &crypto.Digest256{testData}
	verifyResult := crypto.Verify(signer.Pubkey, digest, sig)
	if !verifyResult {
		t.Errorf("verify failed")
	}
}
