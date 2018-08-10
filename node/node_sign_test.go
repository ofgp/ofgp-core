package node

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/ofgp/ofgp-core/primitives"
	pb "github.com/ofgp/ofgp-core/proto"
)

func TestSyncMap(t *testing.T) {
	var m sync.Map
	for i := 0; i < 10; i++ {
		m.Store(i, i)
	}
	m.Range(func(k, v interface{}) bool {
		fmt.Printf("k:%v,v:%v", k, v)
		val := v.(int)
		if val == 7 {
			m.Delete(val)
		}
		return true
	})
	fmt.Println()
	m.Range(func(k, v interface{}) bool {
		fmt.Printf("k:%v,v:%v", k, v)
		return true
	})
}

func TestCheckSignTimeout(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "braft_synced")
	if err != nil {
		log.Fatalf("create tmp dir err:%v", err)
	}
	db, _ := dgwdb.NewLDBDatabase(tempDir, cluster.DbCache, cluster.DbFileHandles)

	primitives.InitDB(db, primitives.GenesisBlockPack)
	bs := primitives.NewBlockStore(db, nil, nil, nil, nil, nil, 0)
	ts := primitives.NewTxStore(db)
	go ts.Run(context.TODO())
	node := &BraftNode{
		blockStore: bs,
		txStore:    ts,
	}

	for i := 0; i < 2; i++ {
		idStr := strconv.Itoa(i)
		req := &pb.SignTxRequest{
			WatchedTx: &pb.WatchedTxInfo{
				Txid: idStr,
			},
			Time: 1,
		}
		primitives.SetSignMsg(db, req, req.WatchedTx.Txid)
		node.signedResultCache.Store(idStr, &SignedResultCache{
			initTime: 1,
		})
	}
	node.waitingConfirmTxs = make(map[string]*waitingConfirmTx)
	node.waitingConfirmTxs["1"] = &waitingConfirmTx{}
	node.checkSignTimeout()
	if primitives.GetSignMsg(db, "1") != nil {
		t.Error("del fail")
	}
	time.Sleep(time.Second)
	for _, tx := range ts.GetFreshWatchedTxs() {
		t.Logf("readd watched id:%s", tx.Tx.Txid)
	}

}

func TestCache(t *testing.T) {
	cache := &SignedResultCache{
		cache:       make(map[int32]*pb.SignedResult),
		totalCount:  0,
		signedCount: 0,
		initTime:    time.Now().Unix(),
	}
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			cache.addTotalCount()
			if cache.setDone() {
				t.Log("set done")
			}
		}()
	}
	wg.Wait()
	t.Logf("totalcnt:%d", cache.getTotalCount())
}
