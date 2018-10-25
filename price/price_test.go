package price_test

import (
	"testing"

	"github.com/ofgp/ofgp-core/price"
)

func TestGetCurrPrice(t *testing.T) {
	tool := price.NewPriceTool("http://127.0.0.1:8000")
	res, err := tool.GetCurrPrice("BCH-USDT")
	if err != nil {
		t.Error("get curr price failed, err: ", err)
	}
	if len(res.Err) != 0 {
		t.Error("get curr price failed, err: ", res.Err)
	} else {
		t.Logf("get price: %v %v", res.Price, res.Timestamp)
	}
}

func TestGetPriceByTs(t *testing.T) {
	tool := price.NewPriceTool("http://127.0.0.1:8000")
	res, err := tool.GetPriceByTimestamp("BCH-USDT", 1539344897)
	if err != nil {
		t.Error("get curr price failed, err: ", err)
	}
	if len(res.Err) != 0 {
		t.Error("get curr price failed, err: ", res.Err)
	} else {
		t.Logf("get price: %v %v", res.Price, res.Timestamp)
	}
}
