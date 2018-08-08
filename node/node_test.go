package node

import (
	"bitcoinWatcher/coinmanager"
	btcwatcher "bitcoinWatcher/mortgagewatcher"
	"dgateway/cluster"
	pb "dgateway/proto"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
)

func Init(tmpDir string) {
	viper.Set("KEYSTORE.count", 1)
	viper.Set("KEYSTORE.keystore_private_key", "C51C9CB7A7EC9D12BB37B3700856690719A44056B750AB03A21247A4903BF3CB")
	viper.Set("KEYSTORE.service_id", "0daf7126-ebbb-4b2d-86f8-a480c1fd45a8")
	viper.Set("KEYSTORE.url", "http://47.98.185.203:8976")
	viper.Set("KEYSTORE.redeem", "524104a2e82be35d90d954e15cc5865e2f8ac22fd2ddbd4750f4bfc7596363a3451d1b75f4a8bad28cf48f63595349dbc141d6d6e21f4feb65bdc5e1a8382a2775e78741049fd6230e3badbbc7ba190e10b2fc5c3d8ea9b758a43e98ab2c8f83c826ae7eabea6d88880bc606fa595cd8dd17fc7784b3e55d8ee0705045119545a803215b8041044667e5b36f387c4d8d955c33fc271f46d791fd3433c0b2f517375bbd9aae6b8c2392229537b109ac8eadcce104aeaa64db2d90bef9008a09f8563cdb05ffb60b53ae")
	viper.Set("KEYSTORE.deferation_address", "bchreg:pzzx7c5l0zlxyde36gwkhm7ua86qlvk90y2ecnynug")
	viper.Set("KEYSTORE.hash_0", "3722834BCB13F7308C28907B69A99DB462F39036")
	viper.Set("KEYSTORE.key_0", "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787")
	viper.Set("KEYSTORE.local_pubkey_hash", "3722834BCB13F7308C28907B69A99DB462F39036")
	viper.Set("DGW.dbpath", tmpDir)
	viper.Set("DGW.count", 1)
	viper.Set("DGW.local_id", 0)
	viper.Set("DGW.bch_height", 100)
	viper.Set("DGW.btc_height", 100)
	viper.Set("DGW.eth_height", 100)
	viper.Set("DGW.local_p2p_port", 10000)
	viper.Set("DGW.local_http_port", 8080)
	viper.Set("DGW.host_0", "127.0.0.1:10000")
	viper.Set("DGW.status_0", true)
	viper.Set("DGW.eth_confirm_count", 6)
	viper.Set("DGW.eth_client_url", "ws://47.98.185.203:8830")
	viper.Set("coin_type", "bch")
	viper.Set("net_param", "regtest")
	viper.Set("BCH.rpc_server", "47.97.167.221:8445")
	viper.Set("BCH.rpc_user", "tanshaohua")
	viper.Set("BCH.rpc_password", "hahaha")
	viper.Set("BCH.confirm_block_num", 1)
	viper.Set("BCH.coinbase_confirm_block_num", 100)

	cluster.Init()
}

func TestStartWatcher(t *testing.T) {
	viper.SetConfigFile("/Users/bitmain/workspace/src/dgateway/config.toml")
	err := viper.ReadInConfig()
	if err != nil {
		t.Errorf("read config err:%v", err)
		return
	}
	rpcServer := viper.Get("BCH.rpc_server")
	t.Logf("rpcserver is :%s", rpcServer)
	cluster.Init()
	pubkeyList := cluster.GetPubkeyList()
	bchFederationAddress, bchRedeem, err := coinmanager.GetMultiSigAddress(pubkeyList, cluster.QuorumN, "bch")
	_, err = btcwatcher.NewMortgageWatcher("bch", viper.GetInt64("DGW.bch_height"),
		bchFederationAddress, bchRedeem, defaultUtxoLockTime)
	if err != nil {
		panic(fmt.Sprintf("new bitcoin watcher failed, err: %v", err))
	}
	// bchWatcher.StartWatch()
	// ch := bchWatcher.GetTxChan()
	// tx := <-ch
	// t.Logf("get tx from ch:%v", tx)
}

func TestNode(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "braft")
	if err != nil {
		t.Fatalf("create tempdir failed, err: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	Init(tmpDir)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", viper.Get("DGW.local_p2p_port")))
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	bn := NewBraftNode(cluster.NodeList[0])
	if bn == nil {
		t.Fatal("failed to create node")
	}
	defer bn.Stop()
	pb.RegisterBraftServer(grpcServer, bn)
	go func() {
		grpcServer.Serve(lis)
	}()
	bn.Run()
	time.Sleep(5 * time.Second)
	if !bn.leader.isInCharge() {
		t.Errorf("leader election failed")
	}
	term := bn.blockStore.GetNodeTerm()
	if term != 0 {
		t.Error("node term is not 0")
	}

	testNodeBasic(t, bn)

	testAccuse(t, bn, term)
	grpcServer.Stop()
}

func testAccuse(t *testing.T, bn *BraftNode, term int64) {
	// 首先发起一个accuse事件, 然后等待再次选举
	bn.blockStore.NewWeakAccuseEvent.Emit(term)
	time.Sleep(15 * time.Second)
	if !bn.leader.isInCharge() {
		t.Error("leader re-election failed")
	}
	if bn.blockStore.GetNodeTerm() != 1 {
		t.Error("node term is not 1")
	}

	testNodeBasic(t, bn)
}

func testNodeBasic(t *testing.T, bn *BraftNode) {
	currHeight := bn.blockStore.GetCommitHeight()
	if currHeight <= 0 {
		t.Errorf("commit height not greater than 0")
	}
	block := bn.blockStore.GetCommitByHeight(currHeight)
	if block == nil {
		t.Error("get committed block failed")
	}
	if !bn.blockStore.IsCommitted(block.BlockId()) {
		t.Error("IsCommitted check failed")
	}
	if bn.blockStore.GetVotie() == nil {
		t.Error("GetVotie check failed")
	}
}

// TODO 测试监听交易是否正常
func testTx(t *testing.T, bn *BraftNode) {

}

func TestCheckOnChain(t *testing.T) {
	bn := &BraftNode{}
	bn.waitingConfirmTxs = map[string]*waitingConfirmTx{
		"1": &waitingConfirmTx{
			msgId:     "1",
			chainType: "btc",
			chainTxId: "1_chain",
			timestamp: time.Now(),
			TokenTo:   1,
		},
		"2": &waitingConfirmTx{
			msgId:     "2",
			chainType: "btc",
			chainTxId: "1_chain",
			TokenTo:   1,
			timestamp: time.Now(),
		},
	}
	CheckOnChainInterval = 1
	CheckOnChainCur = 2
	go func() {
		time.Sleep(time.Second)
		bn.mu.Lock()
		defer bn.mu.Unlock()
		bn.waitingConfirmTxs["3"] = &waitingConfirmTx{
			msgId:     "3",
			chainType: "btc",
			chainTxId: "3_chain",
			timestamp: time.Now(),
			TokenTo:   1,
		}
	}()
	bn.watchWatingConfirmTx(context.TODO())
}
