package crypto

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ofgp/ofgp-core/util/assert"
)

type signBody struct {
	InputHex   string `json:"inputHex"`
	PubHashHex string `json:"pubHashHex"`
	ServiceId  string `json:"serviceId"`
	Timestamp  int64  `json:"timestamp"`
}

type sigContent struct {
	R               string `json:"r"`
	S               string `json:"s"`
	SignatureDerHex string `json:"signatureDerHex"`
}

type signResponse struct {
	Code   int        `json:"code"`
	Data   sigContent `json:"data"`
	ErrMsg string     `json:"msg"`
}

type SecureSigner struct {
	Pubkey     []byte
	PubKeyHex  string
	PubkeyHash string
	ksSigner   *Signer
	ServiceId  string
	KsUrl      string
	KsPrivKey  string
}

// NewSecureSigner 通过pubkeyhex和pubkeyhash生成一个新的SecureSigner对象。
// pubkeyHex是非压缩的公钥
func NewSecureSigner(pubkeyHex, pubkeyHash string) *SecureSigner {
	data, err := hex.DecodeString(pubkeyHex)
	assert.ErrorIsNil(err)
	return &SecureSigner{
		Pubkey:     data,
		PubKeyHex:  pubkeyHex,
		PubkeyHash: pubkeyHash,
	}
}

var client *http.Client

func init() {
	client = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   800 * time.Millisecond,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
		Timeout: 1000 * time.Millisecond,
	}
}

// InitKeystoreParam 初始化访问keystore需要的私钥，仅需调用一次
func (signer *SecureSigner) InitKeystoreParam(ksPrivKey string, serviceId string, url string) {
	signer.ksSigner = SignerFromText(ksPrivKey)
	signer.KsPrivKey = ksPrivKey
	signer.ServiceId = serviceId
	signer.KsUrl = url
}

// Sign 访问keystore进行数据加签
func (signer *SecureSigner) Sign(digest []byte) ([]byte, error) {
	url := strings.Join([]string{signer.KsUrl, "key/sign"}, "/")
	sig, err := dialKeyStore(digest, signer.ksSigner, signer.ServiceId, url, signer.PubkeyHash)
	if err != nil {
		return nil, err
	}
	if sig.Code != 0 {
		return nil, fmt.Errorf("keystore sign failed, err: %s", sig.ErrMsg)
	}
	return hex.DecodeString(sig.Data.SignatureDerHex)
}

func createReq(url string, jsonBody []byte, headerData, serviceID string) *http.Request {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Printf("create sign req err:%v\n", req)
		return nil
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	req.Header.Set("signature", headerData)
	req.Header.Set("serviceId", serviceID)
	req.Close = true
	return req
}

func dialKeyStore(digest []byte, ksSigner *Signer, serviceId string, url string, pubkeyHash string) (*signResponse, error) {
	data := hex.EncodeToString(digest)
	body := signBody{
		InputHex:   data,
		PubHashHex: pubkeyHash,
		ServiceId:  serviceId,
		Timestamp:  time.Now().Unix(),
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	signedData, err := ksSigner.Sign(jsonBody)
	headerData := hex.EncodeToString(signedData)
	req := createReq(url, jsonBody, headerData, serviceId)
	var cnt int
	res, err := client.Do(req)
	for err != nil && cnt < 2 {
		req = createReq(url, jsonBody, headerData, serviceId)
		res, err = client.Do(req)
		cnt++
	}

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	sig := new(signResponse)
	err = json.Unmarshal(resBody, sig)
	if err != nil {
		return nil, err
	}
	return sig, nil
}
