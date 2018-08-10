package cluster_test

import (
	"strconv"
	"testing"

	"github.com/ofgp/ofgp-core/cluster"
	pb "github.com/ofgp/ofgp-core/proto"

	"github.com/spf13/viper"
)

func init() {
	viper.Set("KEYSTORE.count", 3)
	viper.Set("KEYSTORE.local_pubkey_hash", "3722834BCB13F7308C28907B69A99DB462F39036")
	viper.Set("DGW.local_id", 0)
	viper.Set("KEYSTORE.key_0", "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787")
	viper.Set("KEYSTORE.key_1", "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787")
	viper.Set("KEYSTORE.key_2", "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787")
	viper.Set("DGW.host_0", "127.0.0.1:10000")
	viper.Set("DGW.host_1", "127.0.0.1:10001")
	viper.Set("DGW.host_2", "127.0.0.1:10002")
	viper.Set("DGW.status_0", true)
	viper.Set("DGW.status_1", true)
	viper.Set("DGW.status_2", true)
}

func doRealTest(t *testing.T) {
	if cluster.ClusterSize != 3 || cluster.TotalNodeCount != 3 {
		t.Fatalf("cluster Init failed, cluster size: %d, total node count: %d", cluster.ClusterSize, cluster.TotalNodeCount)
	}
	if cluster.MaxFaultyN != 0 || cluster.QuorumN != 2 || cluster.AccuseQuorumN != 1 {
		t.Error("cluster params not correct")
	}
	if cluster.LeaderNodeOfTerm(0) != 0 || cluster.LeaderNodeOfTerm(1) != 1 || cluster.LeaderNodeOfTerm(2) != 2 ||
		cluster.LeaderNodeOfTerm(3) != 0 {
		t.Error("test LeaderNodeOfTerm failed")
	}

	cluster.AddNode("127.0.0.1:10003", 3, "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787", "")
	if cluster.ClusterSize != 4 || cluster.TotalNodeCount != 4 {
		t.Fatalf("cluster Init failed, cluster size: %d, total node count: %d", cluster.ClusterSize, cluster.TotalNodeCount)
	}
	if cluster.MaxFaultyN != 1 || cluster.QuorumN != 3 || cluster.AccuseQuorumN != 2 {
		t.Error("cluster params not correct")
	}
	if cluster.LeaderNodeOfTerm(0) != 0 || cluster.LeaderNodeOfTerm(1) != 1 || cluster.LeaderNodeOfTerm(2) != 2 ||
		cluster.LeaderNodeOfTerm(3) != 3 || cluster.LeaderNodeOfTerm(4) != 0 {
		t.Error("test LeaderNodeOfTerm failed")
	}

	cluster.DeleteNode(1)
	if cluster.ClusterSize != 3 || cluster.TotalNodeCount != 4 {
		t.Fatalf("cluster Init failed, cluster size: %d, total node count: %d", cluster.ClusterSize, cluster.TotalNodeCount)
	}
	if cluster.MaxFaultyN != 0 || cluster.QuorumN != 2 || cluster.AccuseQuorumN != 1 {
		t.Error("cluster params not correct")
	}
	if cluster.LeaderNodeOfTerm(0) != 0 || cluster.LeaderNodeOfTerm(1) != 2 || cluster.LeaderNodeOfTerm(2) != 2 ||
		cluster.LeaderNodeOfTerm(3) != 3 || cluster.LeaderNodeOfTerm(4) != 0 {
		t.Error("test LeaderNodeOfTerm failed")
	}

	cluster.RecoverNode(1)
	if cluster.ClusterSize != 4 || cluster.TotalNodeCount != 4 {
		t.Fatalf("cluster Init failed, cluster size: %d, total node count: %d", cluster.ClusterSize, cluster.TotalNodeCount)
	}
	if cluster.MaxFaultyN != 1 || cluster.QuorumN != 3 || cluster.AccuseQuorumN != 2 {
		t.Error("cluster params not correct")
	}
	if cluster.LeaderNodeOfTerm(0) != 0 || cluster.LeaderNodeOfTerm(1) != 1 || cluster.LeaderNodeOfTerm(2) != 2 ||
		cluster.LeaderNodeOfTerm(3) != 3 || cluster.LeaderNodeOfTerm(4) != 0 {
		t.Error("test LeaderNodeOfTerm failed")
	}
}

func TestClusterWithConfig(t *testing.T) {
	cluster.Init()
	doRealTest(t)
}

func TestClusterWithNodeList(t *testing.T) {
	nodeList := new(pb.NodeList)
	for i := 0; i < 3; i++ {
		nodeList.NodeList = append(nodeList.NodeList, &pb.NodeInfo{
			NodeId:   int32(i),
			Name:     "server" + strconv.Itoa(i),
			Host:     "127.0.0.1:" + strconv.Itoa(10000+i),
			Pubkey:   "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787",
			IsNormal: true,
		})
	}
	cluster.InitWithNodeList(nodeList)
	doRealTest(t)
}

func TestMultiSig(t *testing.T) {
	sig1 := cluster.MultiSigInfo{
		BtcAddress:      "btc_address1",
		BtcRedeemScript: []byte("btc_redeem1"),
		BchAddress:      "bch_address1",
		BchRedeemScript: []byte("bch_redeem1"),
	}
	cluster.SetCurrMultiSig(sig1)
	if !sig1.Equal(cluster.CurrMultiSig) {
		t.Fatal("multisig not equal")
	}
	cluster.AddMultiSigInfo(cluster.CurrMultiSig)
	if len(cluster.MultiSigSnapshot.SigInfos) != 1 {
		t.Fatal("add multisig failed")
	}
	if !sig1.Equal(cluster.MultiSigSnapshot.SigInfos[0]) {
		t.Fatal("multisig not equal")
	}
	sig2 := cluster.MultiSigInfo{
		BtcAddress:      "btc_address2",
		BtcRedeemScript: []byte("btc_redeem2"),
		BchAddress:      "bch_address2",
		BchRedeemScript: []byte("bch_redeem2"),
	}
	cluster.SetCurrMultiSig(sig2)
	if !sig2.Equal(cluster.CurrMultiSig) {
		t.Fatal("multisig not equal")
	}
	cluster.AddMultiSigInfo(cluster.CurrMultiSig)
	if len(cluster.MultiSigSnapshot.SigInfos) != 2 {
		t.Fatal("add multisig failed")
	}
	if !sig1.Equal(cluster.MultiSigSnapshot.SigInfos[0]) || !sig2.Equal(cluster.MultiSigSnapshot.SigInfos[1]) {
		t.Fatal("multisig not equal")
	}
	latest, _ := cluster.MultiSigSnapshot.GetLatestSigInfo()
	t.Logf("latestInfo:%s", latest)
}
