package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/spf13/viper"
)

type generateInput struct {
	Count     int    `json:"count"`
	ServiceID string `json:"serviceId"`
	TimeStamp int64  `json:"timestamp"`
}

type PubkeyData struct {
	PubHashHex string `json:"pubHashHex"`
	PubHex     string `json:"pubHex"`
}

type GenerateData struct {
	Keys []*PubkeyData `json:"keys"`
}

type GenerateResponse struct {
	Code int `json:"code"`
	Data *GenerateData
	Msg  string `json:"msg"`
}

func calcHash(buf []byte, hasher hash.Hash) []byte {
	hasher.Write(buf)
	return hasher.Sum(nil)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s [configfile]\n", os.Args[0])
		return
	}
	viper.SetConfigFile(os.Args[1])
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("read config failed, err:%v\n", err)
		return
	}

	serviceID := viper.GetString("KEYSTORE.service_id")
	inputData := generateInput{
		Count:     1,
		ServiceID: serviceID,
		TimeStamp: time.Now().Unix(),
	}
	postData, err := json.Marshal(inputData)
	if err != nil {
		fmt.Printf("marshal post data failed, err:%v\n", err)
		return
	}

	keyStorePK := viper.GetString("KEYSTORE.keystore_private_key")
	keypkByte, err := hex.DecodeString(keyStorePK)
	if err != nil {
		fmt.Printf("decode keystore key failed, err: %v\n", err)
		return
	}
	keyPk, _ := btcec.PrivKeyFromBytes(btcec.S256(), keypkByte)
	postDataHash := calcHash(postData, sha256.New())
	postSign, err := keyPk.Sign(postDataHash)
	if err != nil {
		fmt.Printf("sign post data failed, err: %v\n", err)
		return
	}

	url := strings.Join([]string{viper.GetString("KEYSTORE.url"), "key/generate"}, "/")
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(postData))
	if err != nil {
		fmt.Printf("create sign req err:%v\n", req)
		return
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	req.Header.Set("signature", hex.EncodeToString(postSign.Serialize()))
	req.Header.Set("serviceId", serviceID)

	client := &http.Client{}
	rsp, err := client.Do(req)
	if err != nil {
		fmt.Printf("http req failed, err: %v\n", err)
		return
	}
	defer rsp.Body.Close()
	rspData, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		fmt.Printf("read http rsp failed, err: %v\n", err)
		return
	}

	var gd GenerateResponse
	json.Unmarshal(rspData, &gd)
	if gd.Code != 0 {
		fmt.Printf("generate failed, errcode: %d\n", gd.Code)
	} else {
		fmt.Printf("pubkey: %s hash: %s\n", gd.Data.Keys[0].PubHex, gd.Data.Keys[0].PubHashHex)
	}
}
