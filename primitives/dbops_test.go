package primitives

import (
	"dgateway/crypto"
	"dgateway/dgwdb"
	pb "dgateway/proto"
	"dgateway/util"
	"testing"
	"time"
)

func TestGetByHeight(t *testing.T) {
	db, _ := dgwdb.NewLDBDatabase("/Users/bitmain/leveldb_data", 0, 0)
	defer db.Close()
	bps := GetCommitsByHeightSec(db, 0, 3)
	for _, bp := range bps {
		t.Logf("get id:%s height:%v", bp.Id().Data, bp.Height())
	}
}

func TestGetByBlockID(t *testing.T) {
	db, _ := dgwdb.NewLDBDatabase("/Users/bitmain/leveldb_data", 0, 0)
	defer db.Close()
	watchedInfo := &pb.WatchedTxInfo{
		Txid:   "form",
		Amount: 10,
		RechargeList: []*pb.AddressInfo{
			&pb.AddressInfo{
				Address: "to",
				Amount:  10,
			},
		},
		From:      "bch",
		To:        "eth",
		Fee:       1,
		TokenFrom: 1,
		TokenTo:   1,
	}
	tx1 := &pb.Transaction{
		WatchedTx: watchedInfo,
		NewlyTxId: "to",
		Time:      time.Now().Unix(),
	}
	tx1.UpdateId() //网关txid
	txs := []*pb.Transaction{
		tx1,
	}

	preId := new(crypto.Digest256)
	preId.Data = []byte("pre")
	block := &pb.Block{
		TimestampMs:    util.NowMs(),
		PrevBlockId:    preId,
		Type:           pb.Block_TXS,
		Txs:            txs,
		BchBlockHeader: &pb.BchBlockHeader{},
		Reconfig:       &pb.Reconfig{},
		Id:             nil,
	}
	block.UpdateBlockId()
	JustCommitIt(db, &pb.BlockPack{
		Init: &pb.InitMsg{
			Term:   1,
			Height: 1,
			Block:  block,
			NodeId: 0,
			Sig:    []byte("testSig"),
		},
	})
	blockPack := GetBlockByID(db, block.GetId().Data)
	t.Logf("%v", blockPack)
}
