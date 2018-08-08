//+build ignore

package main

import (
	btcfunc "bitcoinWatcher/coinmanager"
	"bytes"
	"dgateway/crypto"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type genInput struct {
	Count     int    `json:"count"`
	ServiceId string `json:"serviceId"`
	Timestamp int64  `json:"timestamp"`
}

type genKeys struct {
	PubHashHex string `json:"pubHashHex"`
	PubHex     string `json:"pubHex"`
}

type genData struct {
	Keys []genKeys `json:"keys"`
}

type genOutput struct {
	Code int     `json:"code"`
	Data genData `json:"data"`
	Msg  string  `json:"msg"`
}

func main() {
	signKey := "C51C9CB7A7EC9D12BB37B3700856690719A44056B750AB03A21247A4903BF3CB"
	signer := crypto.SignerFromText(signKey)
	input := genInput{
		Count:     3,
		ServiceId: "0daf7126-ebbb-4b2d-86f8-a480c1fd45a8",
		Timestamp: time.Now().Unix(),
	}
	data, err := json.Marshal(input)
	if err != nil {
		fmt.Println("json marshal failed", err.Error())
		return
	}
	signedData, err := signer.Sign(data)
	headerData := hex.EncodeToString(signedData)
	client := &http.Client{}
	fmt.Println("headerData:", headerData)
	req, err := http.NewRequest("POST", "http://47.98.185.203:8976/key/generate", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	req.Header.Set("signature", headerData)

	res, err := client.Do(req)
	if err != nil {
		fmt.Println("post failed", err.Error())
		return
	}
	defer res.Body.Close()
	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println("read all failed", err.Error())
		return
	}
	output := new(genOutput)
	err = json.Unmarshal(content, output)
	if output.Code != 0 {
		fmt.Println("gen failed, ", output.Code)
		return
	}
	fmt.Println(output)
	var pubList []string
	for _, k := range output.Data.Keys {
		pubList = append(pubList, k.PubHex)
	}
	address, script, err := btcfunc.GetMultiSigAddress(pubList, 2, "bch")
	fmt.Println("address: ", address)
	fmt.Println("script: ", hex.EncodeToString(script))
}
