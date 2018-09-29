package httpsvr_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"

	"github.com/ofgp/ofgp-core/util"

	pb "github.com/ofgp/ofgp-core/proto"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/ofgp/ofgp-core/httpsvr"
	"github.com/ofgp/ofgp-core/node"
	"github.com/ofgp/ofgp-core/primitives"

	"github.com/golang/protobuf/proto"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var (
	handler *httpsvr.HTTPHandler
	bnode   *node.BraftNode
)

func Init(tmpDir string) {
	viper.Set("KEYSTORE.count", 2)
	viper.Set("KEYSTORE.keystore_private_key", "C51C9CB7A7EC9D12BB37B3700856690719A44056B750AB03A21247A4903BF3CB")
	viper.Set("KEYSTORE.service_id", "0daf7126-ebbb-4b2d-86f8-a480c1fd45a8")
	viper.Set("KEYSTORE.url", "http://47.98.185.203:8976")
	viper.Set("KEYSTORE.redeem", "524104a2e82be35d90d954e15cc5865e2f8ac22fd2ddbd4750f4bfc7596363a3451d1b75f4a8bad28cf48f63595349dbc141d6d6e21f4feb65bdc5e1a8382a2775e78741049fd6230e3badbbc7ba190e10b2fc5c3d8ea9b758a43e98ab2c8f83c826ae7eabea6d88880bc606fa595cd8dd17fc7784b3e55d8ee0705045119545a803215b8041044667e5b36f387c4d8d955c33fc271f46d791fd3433c0b2f517375bbd9aae6b8c2392229537b109ac8eadcce104aeaa64db2d90bef9008a09f8563cdb05ffb60b53ae")
	viper.Set("KEYSTORE.deferation_address", "bchreg:pzzx7c5l0zlxyde36gwkhm7ua86qlvk90y2ecnynug")

	viper.Set("KEYSTORE.hash_0", "3722834BCB13F7308C28907B69A99DB462F39036")
	viper.Set("KEYSTORE.key_0", "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787")
	viper.Set("KEYSTORE.hash_1", "E37B5BEBF46B6CAA4B2146CCD83D61966B33687A")
	viper.Set("KEYSTORE.key_1", "049FD6230E3BADBBC7BA190E10B2FC5C3D8EA9B758A43E98AB2C8F83C826AE7EABEA6D88880BC606FA595CD8DD17FC7784B3E55D8EE0705045119545A803215B80")

	viper.Set("KEYSTORE.local_pubkey_hash", "3722834BCB13F7308C28907B69A99DB462F39036")
	viper.Set("DGW.dbpath", tmpDir)
	viper.Set("DGW.count", 2)
	viper.Set("DGW.local_id", 0)
	viper.Set("DGW.bch_height", 100)
	viper.Set("DGW.eth_height", 100)
	viper.Set("DGW.local_p2p_port", 10000)
	viper.Set("DGW.local_http_port", 8080)

	viper.Set("DGW.host_0", "127.0.0.1:10000")
	viper.Set("DGW.status_0", true)
	viper.Set("DGW.host_1", "127.0.0.1:10001")
	viper.Set("DGW.status_1", true)

	viper.Set("DGW.eth_confirm_count", 6)
	viper.Set("DGW.eth_client_url", "ws://47.98.185.203:8830")
	viper.Set("coin_type", "bch")
	viper.Set("net_param", "regtest")
	viper.Set("BCH.rpc_server", "47.97.167.221:8445")
	viper.Set("BCH.rpc_user", "tanshaohua")
	viper.Set("BCH.rpc_password", "hahaha")
	viper.Set("BCH.confirm_block_num", 1)
	viper.Set("BCH.coinbase_confirm_block_num", 100)
	viper.Set("DGW.start_mode", 3)
	cluster.Init()
	_, bnode = node.RunNew(0, nil)
	grpcServer := grpc.NewServer()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 10001))
	if err != nil {
		log.Fatalf("listen port err:%v", err)
	}
	pb.RegisterBraftServer(grpcServer, bnode)
	go func() {
		grpcServer.Serve(lis)
	}()

	handler = httpsvr.NewHTTPHandler(bnode)
}

//打开目录，获取db
func openDbOrDie(dbPath string) (db *dgwdb.LDBDatabase, newlyCreated bool) {
	if len(dbPath) == 0 {
		homeDir, err := util.GetHomeDir()
		if err != nil {
			panic("Cannot detect the home dir for the current user.")
		}
		dbPath = path.Join(homeDir, "braftdb")
	}

	fmt.Println("open db path ", dbPath)
	info, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		if err := os.Mkdir(dbPath, 0700); err != nil {
			panic(fmt.Errorf("Cannot create db path %v", dbPath))
		}
		newlyCreated = true
	} else {
		if err != nil {
			panic(fmt.Errorf("Cannot get info of %v", dbPath))
		}
		if !info.IsDir() {
			panic(fmt.Errorf("Datavse path (%v) is not a directory", dbPath))
		}
		if c, _ := ioutil.ReadDir(dbPath); len(c) == 0 {
			newlyCreated = true
		} else {
			newlyCreated = false
		}
	}

	db, err = dgwdb.NewLDBDatabase(dbPath, cluster.DbCache, cluster.DbFileHandles)
	if err != nil {
		panic(fmt.Errorf("Failed to open database at %v", dbPath))
	}
	return
}

func createIndex(db *dgwdb.LDBDatabase, txs []*pb.Transaction, height int64) {
	for i, tx := range txs {
		entry := &pb.TxLookupEntry{
			Height: height,
			Index:  int32(i),
		}
		primitives.SetTxLookupEntry(db, tx.Id, entry)
		primitives.SetTxIdMap(db, tx.WatchedTx.Txid, tx.Id)
		primitives.SetTxIdMap(db, tx.NewlyTxId, tx.Id)
	}
}
func createBlockData(db *dgwdb.LDBDatabase, blockStore *primitives.BlockStore, formTxid, toTxid, toaddr string, blcokHight int64) {
	pre := blockStore.GetCommitTop()
	watchedInfo := &pb.WatchedTxInfo{
		Txid:   formTxid,
		Amount: 10,
		RechargeList: []*pb.AddressInfo{
			&pb.AddressInfo{
				Address: toaddr,
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
		NewlyTxId: toTxid,
		Time:      time.Now().Unix(),
	}
	tx1.UpdateId() //网关txid
	txs := []*pb.Transaction{
		tx1,
	}
	var preId *crypto.Digest256
	if pre != nil {
		preId = pre.BlockId()
	} else {
		preId = new(crypto.Digest256)
	}
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
	log.Println("strt commit -----------")
	blockStore.JustCommitIt(&pb.BlockPack{
		Init: &pb.InitMsg{
			Term:   1,
			Height: blcokHight,
			Block:  block,
			NodeId: 0,
			Sig:    []byte("testSig"),
		},
	})
	createIndex(db, txs, blcokHight)
	log.Println("end commit")
	// blockpack := blockStore.GetCommitByHeight(blcokHight)
	// log.Printf("get block is:%v", blockpack)
}
func createTestData(dir string) {
	// db, err := dgwdb.NewLDBDatabase(dbPath, cluster.DbCache, cluster.DbFileHandles)
	db, newlyCreated := openDbOrDie(dir)
	db.DeleteWithPrefix([]byte("B"))
	defer db.Close()
	if newlyCreated {
		primitives.InitDB(db, primitives.GenesisBlockPack)
	}
	blockStore := primitives.NewBlockStore(db, nil, nil, nil, nil, nil, 0)
	for i := 0; i < 2; i++ {
		num := rand.Intn(1000)
		from := getStr(num)
		to := getStr(num + 1)
		addr := getStr(num + 1)
		createBlockData(db, blockStore, from, to, addr, int64(i))
	}
}
func getStr(n int) string {
	h := crypto.NewHasher256()
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, 3)
	h.Feed(bytes)
	digest := h.Sum(nil)
	return hex.EncodeToString(digest.Data)
}

func TestSize(t *testing.T) {
	watchedInfo := &pb.WatchedTxInfo{
		Txid:   "123w",
		Amount: 10,
		RechargeList: []*pb.AddressInfo{
			&pb.AddressInfo{
				Address: "",
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
		NewlyTxId: "",
	}
	tx1.UpdateId() //网关txid
	block := &pb.Block{
		TimestampMs: util.NowMs(),
		Type:        pb.Block_TXS,
		Txs: []*pb.Transaction{
			tx1,
		},
		BchBlockHeader: &pb.BchBlockHeader{},
		Reconfig:       &pb.Reconfig{},
		Id:             nil,
	}
	bytes, _ := proto.Marshal(block)
	t.Logf("the blockpack size is:%d,size2:%d", block.XXX_Size(), len(bytes))
}
func TestCreate(t *testing.T) {
	dir := "/Users/bitmain/leveldb_data_test"
	createTestData(dir)
}

func TestMain(m *testing.M) {
	tmpDir, err := ioutil.TempDir("", "braft")
	if err != nil {
		panic("create tempdir failed")
	}
	createTestData(tmpDir)
	Init(tmpDir)
	code := m.Run()
	defer bnode.Stop()
	defer os.RemoveAll(tmpDir)
	os.Exit(code)
}

func TestGetCurrentBlock(t *testing.T) {
	req, _ := http.NewRequest("GET", "/block/current", nil)
	w := httptest.NewRecorder()
	router := httprouter.New()
	router.GET("/block/current", handler.GetCurrentBlock)

	router.ServeHTTP(w, req)
	t.Logf("get block res:%v", w.Body.String())
	// if status := w.Code; status != http.StatusOK {
	// 	t.Errorf("expected status code: %d, got: %d", http.StatusOK, status)
	// } else {
	// 	t.Log(w.Body.String())
	// }
}

func TestGetBlocks(t *testing.T) {
	req, _ := http.NewRequest("GET", "/blocks?start=1&end=5", nil)
	w := httptest.NewRecorder()
	router := httprouter.New()
	router.GET("/blocks", handler.GetBlocksByHeightSec)

	router.ServeHTTP(w, req)
	t.Logf("get block res:%v", w.Body.String())
}
func TestGetBlockOne(t *testing.T) {
	req, _ := http.NewRequest("GET", "/block/height/0", nil)
	w := httptest.NewRecorder()
	router := httprouter.New()
	router.GET("/block/height/:height", handler.GetBlockByHeight)

	router.ServeHTTP(w, req)
	t.Logf("get blockone res:%v", w.Body.String())
}
func TestGetTxByTxID(t *testing.T) {
	block := bnode.GetBlockCurrent()
	if block == nil || len(block.Txs) == 0 {
		t.Error("current tx is nil")
	}
	tx := block.Txs[0]
	var txid = tx.ToTxHash
	req, _ := http.NewRequest("GET", "/transaciton/"+txid, nil)
	w := httptest.NewRecorder()
	router := httprouter.New()
	router.GET("/transaciton/:txid", handler.GetTransActionByID)

	router.ServeHTTP(w, req)
	t.Logf("get tx res:%v", w.Body.String())
}
func TestGetBlockByID(t *testing.T) {
	block := bnode.GetBlockCurrent()
	if block == nil || len(block.Txs) == 0 {
		t.Error("current tx is nil")
	}
	blockID := block.ID
	t.Logf("blockid is:%s", blockID)
	req, _ := http.NewRequest("GET", "/block/blockID/"+blockID, nil)
	w := httptest.NewRecorder()
	router := httprouter.New()
	router.GET("/block/blockID/:blockID", handler.GetBlockByID)

	router.ServeHTTP(w, req)
	t.Logf("get block res:%v", w.Body.String())
}
func TestGetNodes(t *testing.T) {
	req, _ := http.NewRequest("GET", "/nodes", nil)
	w := httptest.NewRecorder()
	router := httprouter.New()
	router.GET("/nodes", handler.GetNodes)

	router.ServeHTTP(w, req)
	t.Logf("get block res:%v", w.Body.String())
}
func TestGetBlockHeight(t *testing.T) {
	req, _ := http.NewRequest("GET", "/getblockheight", nil)
	w := httptest.NewRecorder()
	router := httprouter.New()
	router.GET("/getblockheight", handler.GetBlockHeight)

	router.ServeHTTP(w, req)
	if status := w.Code; status != http.StatusOK {
		t.Errorf("expected status code: %d, got: %d", http.StatusOK, status)
	} else {
		t.Log(w.Body.String())
	}
}

func TestGetTxBySidechainTxId(t *testing.T) {
	req, _ := http.NewRequest("GET", "/gettransactionbyscid/123", nil)
	w := httptest.NewRecorder()
	router := httprouter.New()
	router.GET("/gettransactionbyscid/:txid", handler.GetTxBySidechainTxId)
	router.ServeHTTP(w, req)
	if status := w.Code; status != http.StatusOK {
		t.Errorf("expected status code: %d, got: %d", http.StatusOK, status)
	} else {
		t.Log(w.Body.String())
	}
}

func TestAddWatchedTx(t *testing.T) {
	watchedTx := &pb.WatchedTxInfo{
		Txid:   "testID",
		Amount: 1024,
		RechargeList: []*pb.AddressInfo{
			&pb.AddressInfo{
				Address: "recharge1",
				Amount:  1024,
			},
		},
		From:      "bch",
		To:        "eth",
		Fee:       512,
		TokenFrom: 1,
		TokenTo:   2,
	}
	data, _ := json.Marshal(watchedTx)
	reqReader := bytes.NewBuffer(data)
	req, _ := http.NewRequest("POST", "/watchedtx", reqReader)
	w := httptest.NewRecorder()
	router := httprouter.New()
	router.POST("/watchedtx", handler.addTx)
	router.ServeHTTP(w, req)
	if status := w.Code; status != http.StatusOK {
		t.Errorf("expected status code: %d, got: %d", http.StatusOK, status)
	} else {
		t.Log(w.Body.String())
	}
}
