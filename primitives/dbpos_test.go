package primitives_test

import (
	"bytes"
	"dgateway/cluster"
	"dgateway/dgwdb"
	"dgateway/primitives"
	"io/ioutil"
	"math/big"
	"os"
	"testing"
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
