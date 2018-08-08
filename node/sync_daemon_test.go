package node

import (
	"dgateway/cluster"
	"dgateway/crypto"
	"dgateway/primitives"
	pb "dgateway/proto"
	"dgateway/util"
	"log"
	"testing"

	"github.com/spf13/viper"
)

var bnode *BraftNode
var syncer *SyncDaemon

// func TestMain(m *testing.M) {
// 	tmpDir, err := ioutil.TempDir("", "braft")
// 	if err != nil {
// 		log.Fatalf("create tmp dir err:%v", err)
// 	}
// 	viper.Set("KEYSTORE.count", 1)
// 	viper.Set("KEYSTORE.keystore_private_key", "C51C9CB7A7EC9D12BB37B3700856690719A44056B750AB03A21247A4903BF3CB")
// 	viper.Set("KEYSTORE.service_id", "0daf7126-ebbb-4b2d-86f8-a480c1fd45a8")
// 	viper.Set("KEYSTORE.url", "http://47.98.185.203:8976")
// 	viper.Set("KEYSTORE.redeem", "524104a2e82be35d90d954e15cc5865e2f8ac22fd2ddbd4750f4bfc7596363a3451d1b75f4a8bad28cf48f63595349dbc141d6d6e21f4feb65bdc5e1a8382a2775e78741049fd6230e3badbbc7ba190e10b2fc5c3d8ea9b758a43e98ab2c8f83c826ae7eabea6d88880bc606fa595cd8dd17fc7784b3e55d8ee0705045119545a803215b8041044667e5b36f387c4d8d955c33fc271f46d791fd3433c0b2f517375bbd9aae6b8c2392229537b109ac8eadcce104aeaa64db2d90bef9008a09f8563cdb05ffb60b53ae")
// 	viper.Set("KEYSTORE.deferation_address", "bchreg:pzzx7c5l0zlxyde36gwkhm7ua86qlvk90y2ecnynug")
// 	viper.Set("KEYSTORE.hash_0", "3722834BCB13F7308C28907B69A99DB462F39036")
// 	viper.Set("KEYSTORE.key_0", "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787")
// 	viper.Set("KEYSTORE.local_pubkey_hash", "3722834BCB13F7308C28907B69A99DB462F39036")
// 	viper.Set("DGW.dbpath", tmpDir)
// 	viper.Set("DGW.count", 1)
// 	viper.Set("DGW.local_id", 0)
// 	viper.Set("DGW.bch_height", 100)
// 	viper.Set("DGW.eth_height", 100)
// 	viper.Set("DGW.local_p2p_port", 10000)
// 	viper.Set("DGW.local_http_port", 8080)
// 	viper.Set("DGW.host_0", "127.0.0.1:10000")
// 	viper.Set("DGW.status_0", true)
// 	viper.Set("DGW.eth_confirm_count", 6)
// 	viper.Set("DGW.eth_client_url", "ws://47.98.185.203:8830")
// 	viper.Set("coin_type", "bch")
// 	viper.Set("net_param", "regtest")
// 	viper.Set("BCH.rpc_server", "47.97.167.221:8445")
// 	viper.Set("BCH.rpc_user", "tanshaohua")
// 	viper.Set("BCH.rpc_password", "hahaha")
// 	viper.Set("BCH.confirm_block_num", 1)
// 	viper.Set("BCH.coinbase_confirm_block_num", 100)
// 	viper.Set("DGW.new_node_pubkey", "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787")
// 	viper.Set("DGW.local_pubkey", "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787")
// 	viper.Set("DGW.new_node_host", "127.0.0.2")
// 	viper.Set("DGW.local_host", "127.0.0.2")
// 	cluster.Init()

// 	bnode = NewBraftNode(cluster.NodeList[0])
// 	signer := cluster.NodeSigners[0]
// 	//创建测试数据
// 	createTestData(bnode.blockStore, signer)
// 	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", viper.Get("DGW.local_p2p_port")))
// 	if err != nil {
// 		panic(fmt.Sprintf("failed to listen: %v", err))
// 	}

// 	grpcServer := grpc.NewServer()
// 	pb.RegisterBraftServer(grpcServer, bnode)
// 	go func() {
// 		grpcServer.Serve(lis)
// 	}()
// 	ctx, cancel := context.WithCancel(context.Background())
// 	go bnode.leader.Run(ctx)
// 	//创建sync实例
// 	nodeID := viper.GetInt32("DGW.local_id")
// 	syncDir, err := ioutil.TempDir("", "braft_synced")
// 	if err != nil {
// 		log.Fatalf("create tmp dir err:%v", err)
// 	}
// 	db, _ := dgwdb.NewLDBDatabase(syncDir, cluster.DbCache, cluster.DbFileHandles)
// 	primitives.InitDB(db, primitives.GenesisBlockPack)

// 	blockStore := primitives.NewBlockStore(db, nil, nil, nil, nil, signer, nodeID)
// 	pm := cluster.NewPeerManager(2)
// 	syncer = NewSyncDaemon(blockStore, pm)
// 	syncer.maxBlockTimeSection = math.MaxInt32
// 	//指定syncer
// 	// bnode.syncDaemon = syncer
// 	exitCode := m.Run()
// 	defer bnode.Stop()
// 	defer cancel()
// 	defer db.Close()
// 	defer os.RemoveAll(tmpDir)
// 	defer os.RemoveAll(syncDir)
// 	os.Exit(exitCode)
// }

func createTestData(bs *primitives.BlockStore, signer *crypto.SecureSigner) {
	pre := bs.GetCommitByHeight(0)

	//create txBlock
	block := pb.CreateTxsBlock(util.NowMs(), pre.BlockId(), nil)
	init := &pb.InitMsg{
		Term:   0,
		Height: pre.Height() + 1,
		Block:  block,
		NodeId: 0,
	}
	sig, err := signer.Sign(init.Id().Data)
	if err != nil {
		leaderLogger.Error("sign block failed", "err", err)
		return
	}
	init.Sig = sig
	fresh := pb.NewBlockPack(init)
	prepare, err := pb.MakePrepareMsg(fresh.BlockInfo(), 0, signer)
	if err != nil {
		log.Fatalf("create parepare msg err:%v", err)
	}
	fresh.Prepares[prepare.NodeId] = prepare
	commit, err := pb.MakeCommitMsg(fresh.BlockInfo(), 0, signer)
	fresh.Commits[commit.NodeId] = commit
	bs.JustCommitIt(fresh)

	//create joinBlock
	blockJoin := pb.CreateJoinReconfigBlock(util.NowMs(), fresh.BlockId(), pb.Reconfig_JOIN, "testhost", 2)
	InitJoin := &pb.InitMsg{
		Term:   0,
		Height: fresh.Height() + 1,
		Block:  blockJoin,
		NodeId: 0,
	}
	sig, err = signer.Sign(InitJoin.Id().Data)
	if err != nil {
		leaderLogger.Error("sign block failed", "err", err)
		return
	}
	InitJoin.Sig = sig
	freshJoin := pb.NewBlockPack(InitJoin)
	prepare, err = pb.MakePrepareMsg(freshJoin.BlockInfo(), 0, signer)
	if err != nil {
		log.Fatalf("create parepare msg err:%v", err)
	}
	freshJoin.Prepares[prepare.NodeId] = prepare
	commit, err = pb.MakeCommitMsg(freshJoin.BlockInfo(), 0, signer)
	freshJoin.Commits[commit.NodeId] = commit
	bs.JustCommitIt(freshJoin)
}
func TestSyncUP(t *testing.T) {
	nodeID := viper.GetInt32("DGW.local_id")
	t.Logf("nodeID is :%d", nodeID)
	syncer.doSyncUp(nodeID)
	height := syncer.blockStore.GetCommitHeight()
	t.Logf("synced height:%d", height)
	if height == 0 {
		t.Error("sync failed")
	}
}

func TestInterval(t *testing.T) {
	a := int64(cluster.BlockInterval / 1e6)
	t.Logf("time is %d", a)
}

func TestSyncBeforeSendJoinReq(t *testing.T) {

	bnode.syncBeforeSendJoinReq(2)
	height := bnode.syncDaemon.blockStore.GetCommitHeight()
	t.Logf("synced height:%d", height)
	if height == 0 {
		t.Error("sync failed")
	}
}

func TestSendSyncReq(t *testing.T) {
	// bnode.localNodeInfo.Id = 1
	// bnode.sendJoinCheckSyncedRequest()
	bnode2 := &BraftNode{
		localNodeInfo: cluster.NodeInfo{
			Id: 1,
		},
	}
	pubkey := "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787"
	hash := "3722834BCB13F7308C28907B69A99DB462F39036"
	cluster.NodeList = []cluster.NodeInfo{
		cluster.NodeInfo{
			Id:        0,
			Name:      "test",
			Url:       "127.0.0.1:10000",
			PublicKey: crypto.NewSecureSigner(pubkey, hash).Pubkey,
			IsNormal:  true,
		},
	}
	bnode2.blockStore = bnode.syncDaemon.blockStore
	bnode2.signer = cluster.NodeSigners[0]
	bnode2.syncDaemon = syncer
	bnode2.sendJoinCheckSyncedRequest()
	height := bnode2.blockStore.GetCommitHeight()
	t.Logf("synced height:%d", height)
	if height == 0 {
		t.Error("sync failed")
	}
}
