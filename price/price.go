package price

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// PriceInfo 币价信息
type PriceInfo struct {
	Price     float32 `json:"price"`
	Timestamp int64   `json:"timestamp"`
	Err       string  `json:"err"`
}

// PriceTool 获取币价的工具
type PriceTool struct {
	url    string
	client *http.Client
}

// NewPriceTool 返回一个PriceTool实例
func NewPriceTool(url string) *PriceTool {
	client := &http.Client{
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

	return &PriceTool{
		url:    url,
		client: client,
	}
}

func (t *PriceTool) dial(url string) (*PriceInfo, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	res := new(PriceInfo)
	err = json.Unmarshal(body, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// GetCurrPrice 获取指定交易对的币价，ex: BCH-USDT
func (t *PriceTool) GetCurrPrice(symbol string) (*PriceInfo, error) {
	url := strings.Join([]string{t.url, "currprice", symbol}, "/")
	return t.dial(url)
}

// GetPriceByTimestamp 获取指定交易对某个时间戳的币价
func (t *PriceTool) GetPriceByTimestamp(symbol string, ts int) (*PriceInfo, error) {
	url := strings.Join([]string{t.url, "pricebyts", symbol, strconv.Itoa(ts)}, "/")
	return t.dial(url)
}
