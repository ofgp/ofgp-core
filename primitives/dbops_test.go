package primitives_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/ofgp/ofgp-core/primitives"
	pb "github.com/ofgp/ofgp-core/proto"
)

func InitDB(dir string) (*dgwdb.LDBDatabase, error) {
	db, err := dgwdb.NewLDBDatabase(dir, 1, 1)
	return db, err
}

func TestSnapshot(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "braft")
	if err != nil {
		t.Fatalf("create tempdir failed, err: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	db, err := InitDB(tmpDir)
	if err != nil {
		t.Fatalf("init db failed, err: %v", err)
	}

	nodeInfo := cluster.NodeInfo{
		Id:        1,
		Name:      "aaa",
		Url:       "127.0.0.1:10000",
		PublicKey: []byte("pubkey"),
		IsNormal:  true,
	}
	snapshot := cluster.Snapshot{
		NodeList:       []cluster.NodeInfo{nodeInfo},
		ClusterSize:    1,
		TotalNodeCount: 1,
		QuorumN:        1,
		AccuseQuorumN:  1,
	}
	primitives.SetClusterSnapshot(db, "testkey", snapshot)
	dbSnapshot := primitives.GetClusterSnapshot(db, "testkey")
	if dbSnapshot == nil {
		t.Fatalf("get snapshot from db failed")
	}
	dbNodeInfo := dbSnapshot.NodeList[0]
	if !(dbNodeInfo.Id == nodeInfo.Id && dbNodeInfo.IsNormal == nodeInfo.IsNormal && dbNodeInfo.Name == nodeInfo.Name &&
		dbNodeInfo.Url == nodeInfo.Url && bytes.Equal(dbNodeInfo.PublicKey, nodeInfo.PublicKey)) {
		t.Error("compare nodeinfo not equal")
	}
	if !(dbSnapshot.ClusterSize == snapshot.ClusterSize && dbSnapshot.TotalNodeCount == snapshot.TotalNodeCount &&
		dbSnapshot.QuorumN == snapshot.QuorumN && dbSnapshot.AccuseQuorumN == snapshot.AccuseQuorumN) {
		t.Error("compare snapshot not equal")
	}

	multiSig := cluster.MultiSigInfo{
		BtcAddress:      "btcaddess",
		BtcRedeemScript: []byte("btc_redeem"),
		BchAddress:      "bchaddress",
		BchRedeemScript: []byte("bch_redeem"),
	}
	multiSigList := []cluster.MultiSigInfo{multiSig}
	primitives.SetMultiSigSnapshot(db, multiSigList)
	dbMultiSigList := primitives.GetMultiSigSnapshot(db)
	if len(dbMultiSigList) == 0 {
		t.Fatal("get multisig snapshot failed")
	}
	dbMultiSig := dbMultiSigList[0]
	if !dbMultiSig.Equal(multiSig) {
		t.Error("conpare multisig failed")
	}
}

func TestETHHeight(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "braft")
	if err != nil {
		t.Fatalf("create tempdir failed, err: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	db, err := InitDB(tmpDir)
	if err != nil {
		t.Fatalf("init db failed, err: %v", err)
	}

	height := &big.Int{}
	height.SetInt64(100000)
	primitives.SetETHBlockHeight(db, height)
	dbHeight := primitives.GetETHBlockHeight(db)
	if height.Cmp(dbHeight) != 0 {
		t.Error("eth height not equal")
	}

	index := 100
	primitives.SetETHBlockTxIndex(db, index)
	dbIndex := primitives.GetETHBlockTxIndex(db)
	if index != dbIndex {
		t.Error("eth index not equal")
	}
}

func TestGetBlocks(t *testing.T) {
	// tmpDir, err := ioutil.TempDir("", "braft")
	// if err != nil {
	// 	t.Fatalf("create tempdir failed, err: %v", err)
	// }
	// defer os.RemoveAll(tmpDir)
	tmpDir := "/Users/bitmain/leveldb_data_test"
	db, err := InitDB(tmpDir)
	if err != nil {
		t.Fatalf("init db failed, err: %v", err)
	}
	bytes := make([]byte, 32)
	for i := 0; i < 32; i++ {
		bytes[i] = byte(i)
	}
	id0, err := crypto.NewDigest256(bytes)
	if err != nil {
		t.Errorf("create err:%v", err)
		return
	}
	block0 := &pb.Block{
		Id: id0,
		Txs: []*pb.Transaction{
			&pb.Transaction{
				Id: id0,
				WatchedTx: &pb.WatchedTxInfo{
					Txid: "txOld0",
				},
			},
		},
	}
	bytes[0] = 1
	id1, _ := crypto.NewDigest256(bytes)
	block1 := &pb.Block{
		Id: id1,
		Txs: []*pb.Transaction{
			&pb.Transaction{
				Id: id1,
				WatchedTx: &pb.WatchedTxInfo{
					Txid: "txOld1",
				},
			},
		},
	}
	blockPack0 := &pb.BlockPack{
		Init: &pb.InitMsg{
			Term:   0,
			Height: 0,
			Block:  block0,
		},
		Prepares: nil,
		Commits:  nil,
	}
	blockPack1 := &pb.BlockPack{
		Init: &pb.InitMsg{
			Term:   0,
			Height: 1,
			Block:  block1,
		},
		Prepares: nil,
		Commits:  nil,
	}
	primitives.JustCommitIt(db, blockPack0)
	primitives.JustCommitIt(db, blockPack1)
	res0 := primitives.GetCommitByHeight(db, 0)
	data0, _ := json.Marshal(res0.Block())
	t.Logf("block0:%s", data0)

	res1 := primitives.GetCommitByHeight(db, 1)
	data1, _ := json.Marshal(res1.Block())
	t.Logf("block1:%s", data1)
}
