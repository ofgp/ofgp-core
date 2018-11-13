package price

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
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
	endpoint string
	client   *http.Client
}

// NewPriceTool 返回一个PriceTool实例
func NewPriceTool(endpoint string) *PriceTool {
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
		endpoint: endpoint,
		client:   client,
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
func (t *PriceTool) GetCurrPrice(symbol string, forth bool) (*PriceInfo, error) {
	tmp, err := url.Parse(strings.Join([]string{t.endpoint, "currprice", symbol}, "/"))
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if forth {
		params.Set("forth", "1")
	} else {
		params.Set("forth", "0")
	}
	tmp.RawQuery = params.Encode()
	return t.dial(tmp.String())
}

// GetPriceByTimestamp 获取指定交易对某个时间戳的币价
func (t *PriceTool) GetPriceByTimestamp(symbol string, ts int64, forth bool) (*PriceInfo, error) {
	tmp, err := url.Parse(strings.Join([]string{t.endpoint, "pricebyts", symbol, strconv.FormatInt(ts, 10)}, "/"))
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if forth {
		params.Set("forth", "1")
	} else {
		params.Set("forth", "0")
	}
	tmp.RawQuery = params.Encode()
	return t.dial(tmp.String())
}
