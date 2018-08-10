package httpsvr

import (
	"encoding/hex"
	"time"

	"github.com/ofgp/ofgp-core/crypto"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/ofgp/ofgp-core/util"
)

type TestInput struct {
	Value string
}

type getBlockHeightResponse struct {
	Height int64 `json:"height"`
}

type TxInfo struct {
	Height           int64
	PackTime         string `json:"pack_time,omitempty"`
	From             string
	To               string
	Amount           int64
	ChainTxId        string           `json:"chain_txid"`
	AddressAmountMap map[string]int64 `json:"address_amount"`
}

func toTxInfo(tx *pb.Transaction, block *pb.BlockInfo) *TxInfo {
	if tx == nil {
		return nil
	}

	rst := new(TxInfo)
	if block != nil {
		rst.Height = block.Height
		rst.PackTime = util.MsToTime(block.Block.TimestampMs).String()
	} else {
		rst.Height = -1
	}
	rst.Amount = tx.WatchedTx.Amount
	rst.ChainTxId = tx.WatchedTx.Txid
	rst.From = tx.WatchedTx.From
	rst.To = tx.WatchedTx.To
	rst.AddressAmountMap = make(map[string]int64)
	for _, am := range tx.WatchedTx.RechargeList {
		rst.AddressAmountMap[am.Address] = am.Amount
	}
	return rst
}

// BlockInfo 转json的结构体
type BlockInfo struct {
	Height   int64
	PackTime string `json:"pack_time"`
	Type     string
	Term     int64
	Id       string
	PrevId   string `json:"prev_id"`
}

func toBlockInfo(block *pb.BlockInfo) *BlockInfo {
	if block == nil {
		return nil
	}
	rst := new(BlockInfo)
	rst.Height = block.Height
	rst.Term = block.Term
	rst.Id = hex.EncodeToString(block.BlockId.Data)
	rst.PrevId = hex.EncodeToString(block.Block.PrevBlockId.Data)
	rst.PackTime = util.MsToTime(block.Block.TimestampMs).String()
	return rst
}

type createTxRequest struct {
	Type             string
	Amount           int64
	ChainTxId        string           `json:"chain_txid"`
	AddressAmountMap map[string]int64 `json:"address_amount"`
}

type createTxResponse struct {
	Code int32
}

func toWatchedTxInfo(req *createTxRequest) *pb.WatchedTxInfo {
	tx := &pb.WatchedTxInfo{
		Txid:   req.ChainTxId,
		Amount: req.Amount,
		From:   "bch",
		To:     "eth",
	}

	for k, v := range req.AddressAmountMap {
		ai := &pb.AddressInfo{
			Amount:  v,
			Address: k,
		}
		tx.RechargeList = append(tx.RechargeList, ai)
	}
	return tx
}

type txMap struct {
	FromTxId string `json:"from_txid"`
	ToTxId   string `json:"to_txid"`
}

type fakeTxInfo struct {
	From       string `json:"from"`
	To         string `json:"to"`
	FromTxHash string `json:"from_tx_hash"`
	ToTxHash   string `json:"to_tx_hash"`
	Amount     int64  `json:"amount"`
	ToAddr     string `json:"to_addr"`
	TokenFrom  uint32 `json:"token_from"`
	TokenTo    uint32 `json:"token_to"`
}

type fakeBlockInfo struct {
	Data []fakeTxInfo `json:"data"`
}

type fakeBlockResponse struct {
	Height int64  `json:"height"`
	Id     string `json:"id"`
}

func toBlockPack(block *fakeBlockInfo, currBlockHash *crypto.Digest256, height int64) *pb.BlockPack {
	var innerTxs []*pb.Transaction
	for _, tx := range block.Data {
		wtx := &pb.WatchedTxInfo{
			Txid:   tx.FromTxHash,
			Amount: tx.Amount,
			From:   tx.From,
			To:     tx.To,
		}
		recharge := &pb.AddressInfo{
			Address: tx.ToAddr,
			Amount:  tx.Amount,
		}
		wtx.RechargeList = append(wtx.RechargeList, recharge)
		innerTx := &pb.Transaction{
			WatchedTx: wtx,
			NewlyTxId: tx.ToTxHash,
			Time:      time.Now().Unix(),
		}
		innerTx.UpdateId()
		innerTxs = append(innerTxs, innerTx)
	}
	newBlock := pb.CreateTxsBlock(util.NowMs(), currBlockHash, innerTxs)
	init := &pb.InitMsg{
		Term:   1,
		Height: height + 1,
		Block:  newBlock,
		NodeId: 0,
	}
	bp := pb.NewBlockPack(init)
	return bp
}
