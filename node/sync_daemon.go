package node

import (
	"dgateway/cluster"
	"dgateway/dgwdb"
	"dgateway/log"
	"dgateway/primitives"
	pb "dgateway/proto"
	"dgateway/util/assert"
	"fmt"
	"sync"

	"github.com/spf13/viper"
	context "golang.org/x/net/context"
)

var (
	sdLogger = log.New(viper.GetString("loglevel"), "sync")
)

// SyncDaemon 后台向其他节点同步的对象，单独goroutine运行
type SyncDaemon struct {
	mu                  sync.Mutex
	isSyncUp            map[int32]bool
	peerToSyncChan      chan int32
	blockStore          *primitives.BlockStore
	peerManager         *cluster.PeerManager
	maxBlockTimeSection int64 //最大的两个block块之间的时间差
	db                  *dgwdb.LDBDatabase
}

// NewSyncDaemon 新建一个SyncDaemon对象并返回
func NewSyncDaemon(db *dgwdb.LDBDatabase, bs *primitives.BlockStore, pm *cluster.PeerManager) *SyncDaemon {
	assert.True(bs != nil && pm != nil)
	sd := &SyncDaemon{
		db:                  db,
		isSyncUp:            make(map[int32]bool),
		peerToSyncChan:      make(chan int32),
		blockStore:          bs,
		peerManager:         pm,
		maxBlockTimeSection: 2 * int64(cluster.BlockInterval/1e6),
	}

	return sd
}

// SignalSyncUp 通知syncdaemon发起同步请求
func (sd *SyncDaemon) SignalSyncUp(nodeId int32) {
	if nodeId == sd.peerManager.LocalNodeId() {
		return
	}
	sd.peerToSyncChan <- nodeId
}

// Run 循环处理事件的入口函数
func (sd *SyncDaemon) Run(ctx context.Context) {
	for {
		select {
		case nodeId := <-sd.peerToSyncChan:
			var ignore bool
			sd.mu.Lock()
			if sd.isSyncUp[nodeId] {
				ignore = true
			} else {
				ignore = false
				sd.isSyncUp[nodeId] = true
			}
			sd.mu.Unlock()

			if !ignore {
				go sd.doSyncUp(nodeId)
			}
		case <-ctx.Done():
			return

		}
	}
}

// Normal watch模式同步数据
func (sd *SyncDaemon) doSyncUp(nodeId int32) error {
	defer func() {
		sd.mu.Lock()
		sd.isSyncUp[nodeId] = false
		sd.mu.Unlock()
		sdLogger.Debug("Sync up done", "nodeId", nodeId)
	}()

	sdLogger.Debug("start sync up", "nodeId", nodeId)
	if sd == nil {
		return nil
	}
	if sd.peerManager.GetNode(nodeId) == nil {
		return fmt.Errorf("no peerNode found by nodeID:%d", nodeId)
	}

	for {
		req := &pb.SyncUpRequest{
			BaseHeight: sd.blockStore.GetCommitHeight(),
		}
		var (
			rsp *pb.SyncUpResponse
			err error
		)
		if startMode != cluster.ModeWatch {
			rsp, err = sd.peerManager.SyncUp(nodeId, req)
		} else {
			rsp, err = sd.peerManager.WatchSyncUp(nodeId, req)
		}
		if err != nil {
			sdLogger.Error(err.Error())
			return err
		}
		if startMode != cluster.ModeWatch {
			err = sd.processSyncUpResponse(rsp)
		} else {
			err = sd.processSimpleSyncUpResponse(rsp)
		}
		if err != nil {
			return err
		}
		if !rsp.More {
			nodeLogger.Info("sync finished from nodeID", "nodeID", nodeId)
			return nil
		}
	}
}

//doJoinSyncUp  join模式同步数据
func (sd *SyncDaemon) doJoinSyncUp(nodeId int32) error {
	defer func() {
		sd.mu.Lock()
		sd.isSyncUp[nodeId] = false
		sd.mu.Unlock()
		sdLogger.Debug("Sync up done", "nodeId", nodeId)
	}()

	sdLogger.Debug("start sync up", "nodeId", nodeId)
	if sd == nil {
		return nil
	}
	if sd.peerManager.GetNode(nodeId) == nil {
		return fmt.Errorf("no peerNode found by nodeID:%d", nodeId)
	}

	for {
		req := &pb.SyncUpRequest{
			BaseHeight: sd.blockStore.GetCommitHeight(),
		}
		var (
			rsp *pb.SyncUpResponse
			err error
		)
		rsp, err = sd.peerManager.WatchSyncUp(nodeId, req)
		if err != nil {
			sdLogger.Error(err.Error())
			return err
		}
		err = sd.processSimpleSyncUpResponse(rsp)
		if err != nil {
			return err
		}
		if !rsp.More {
			nodeLogger.Info("sync finished from nodeID", "nodeID", nodeId)
			return nil
		}
	}
}

//checkblock 是否合法
func (sd *SyncDaemon) checkBlockPack(blockPack *pb.BlockPack) bool {
	if blockPack == nil || blockPack.Block() == nil {
		sdLogger.Error("get blockPack nil")
		return false
	}
	//check block结构
	if !checkInitMsg(blockPack.Init) {
		sdLogger.Error("check initmsg fail")
		return false
	}
	term := blockPack.Term()
	height := blockPack.Height()
	blockID := blockPack.BlockId()
	//recal blockID
	blockPack.Block().UpdateBlockId()
	if !blockID.EqualTo(blockPack.BlockId()) {
		sdLogger.Error("block id is not equal")
		return false
	}
	if !checkCommits(term, height, blockID, blockPack.Commits) {
		sdLogger.Error("check commits fail")
		return false
	}

	//check block context
	preID := blockPack.PrevBlockId()
	if !sd.blockStore.IsCommitted(preID) {
		sdLogger.Error("pre block not exist", "preID", preID)
		return false
	}
	//check 是否已经commited
	if sd.blockStore.IsCommitted(blockID) {
		sdLogger.Error("duplicate block", "blockID", blockID)
		return false
	}
	preBlock := sd.blockStore.GetBlockByHash(preID)
	if blockPack.Height() != preBlock.Height()+1 {
		sdLogger.Error("block's hight != preheight+1", "preHeight", preBlock.Height(), "height", blockPack.Height())
		return false
	}
	//check 多数派证明
	cnt := sd.peerManager.GetCommitedCnt(blockPack.BlockId())
	if cnt < cluster.AccuseQuorumN {
		sdLogger.Error("block is not commited in accuseQuorum", "cmmitedCnt", cnt)
		return false
	}
	return true
}
func (sd *SyncDaemon) processSyncUpResponse(rsp *pb.SyncUpResponse) error {
	bs := sd.blockStore
	for _, block := range rsp.Commits {
		if !sd.checkBlockPack(block) {
			break
		}
		bs.JustCommitIt(block)
		bs.NewCommittedEvent.Emit(block)
	}

	if rsp.Fresh != nil && rsp.Fresh.Height() > bs.GetCommitHeight() {
		if rsp.Fresh.Init != nil {
			bs.HandleInitMsg(rsp.Fresh.Init)
		}
		for _, prepare := range rsp.Fresh.Prepares {
			bs.HandlePrepareMsg(prepare)
		}
		for _, commit := range rsp.Fresh.Commits {
			bs.HandleCommitMsg(commit)
		}
	}

	if rsp.StrongAccuse != nil {
		bs.HandleStrongAccuse(rsp.StrongAccuse)
	}
	return nil
}

// processSimpleSyncUpResponse 同步数据不参与共识
func (sd *SyncDaemon) processSimpleSyncUpResponse(rsp *pb.SyncUpResponse) error {
	bs := sd.blockStore
	for _, block := range rsp.Commits {
		if !sd.checkBlockPack(block) {
			sdLogger.Error("check block not pass")
			break
		}
		bs.JustCommitIt(block)
		height := block.Height()
		txs := block.Block().Txs
		if len(txs) > 0 {
			for idx, tx := range txs {
				entry := &pb.TxLookupEntry{
					Height: height,
					Index:  int32(idx),
				}
				sdLogger.Debug("simple sync", "txid", tx.WatchedTx.Txid)
				primitives.SetTxLookupEntry(sd.db, tx.Id, entry)
				//from链 tx_id 和网关tx_id的对应
				primitives.SetTxIdMap(sd.db, tx.WatchedTx.Txid, tx.Id)
				//to链和tx_id 和网关tx_id的对应
				primitives.SetTxIdMap(sd.db, tx.NewlyTxId, tx.Id)
			}
		}
	}
	return nil
}
