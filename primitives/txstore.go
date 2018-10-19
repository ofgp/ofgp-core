package primitives

import (
	"context"
	"encoding/hex"
	"sync"
	"time"

	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/dgwdb"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/ofgp/ofgp-core/util"
	"github.com/ofgp/ofgp-core/util/assert"
)

const (
	// 交易的过期时间，过期直接从mempool里面删除
	txTTL = 10 * 60 * time.Second
	// 交易的处理超时时间，超时会发起accuse
	txWaitingTolerance        = 60 * 3 * time.Second
	heartbeatInterval         = 10 * time.Second
	defaultTxWaitingTolerance = 60 * time.Second  //默认交易超时时间
	maxTxWaitingTolerance     = 300 * time.Second //交易最大超时时间
)

const (
	NotExist = iota
	Valid
	Invalid
	CheckChain
)

type addTxsRequest struct {
	txs          []*pb.Transaction
	notAddedChan chan []int
}

func makeAddTxsRequest(txs []*pb.Transaction) addTxsRequest {
	return addTxsRequest{txs, make(chan []int, 1)}
}

// TxQueryResult 保存搜索结果
type TxQueryResult struct {
	Tx      *pb.Transaction
	Height  int64
	BlockID *crypto.Digest256
}

type blockCommitted struct {
	txs    []*pb.Transaction
	height int64
}

func makeBlockCommitted(newBlock *pb.BlockPack) blockCommitted {
	return blockCommitted{newBlock.Block().Txs, newBlock.Height()}
}

type txWithTimeMs struct {
	tx                 *pb.Transaction
	genTs              time.Time
	calcOverdueTs      time.Time
	txWaitingTolerance time.Duration
}

func (t *txWithTimeMs) IsOverdue() bool {
	if t.txWaitingTolerance == 0 {
		t.txWaitingTolerance = defaultTxWaitingTolerance
	}
	return time.Now().After(t.calcOverdueTs.Add(txWaitingTolerance))
}

func (t *txWithTimeMs) IsOutdate() bool {
	return time.Now().After(t.genTs.Add(txTTL))
}

// resetWaitingTolerance 设置超时时间为原先的两倍直到大于最大超时时间
func (t *txWithTimeMs) resetWaitingTolerance() {
	tempTolerance := 2 * t.txWaitingTolerance
	if tempTolerance > maxTxWaitingTolerance {
		tempTolerance = maxTxWaitingTolerance
	}
	t.txWaitingTolerance = tempTolerance
	t.calcOverdueTs = time.Now()
}

// WatchedTxInfo watcher监听到的交易信息
type WatchedTxInfo struct {
	Tx                 *pb.WatchedTxInfo
	genTs              time.Time
	calcOverdueTs      time.Time     //交易创建时间
	txWaitingTolerance time.Duration //交易超时时间
}

func (t *WatchedTxInfo) isOverdue() bool {
	if t.txWaitingTolerance == 0 {
		t.txWaitingTolerance = defaultTxWaitingTolerance
	}
	return time.Now().After(t.calcOverdueTs.Add(t.txWaitingTolerance))

}

// resetWaitingTolerance 设置超时时间为原先的两倍直到大于最大超时时间
func (t *WatchedTxInfo) resetWaitingTolerance() {
	tempTolerance := 2 * t.txWaitingTolerance
	if tempTolerance > maxTxWaitingTolerance {
		tempTolerance = maxTxWaitingTolerance
	}
	t.txWaitingTolerance = tempTolerance
	t.calcOverdueTs = time.Now()
}

func (t *WatchedTxInfo) isOutdate() bool {
	return time.Now().After(t.genTs.Add(txTTL))
}

type txsFetcher struct {
	resultChan chan []*pb.Transaction
}

func makeTxsFetcher() txsFetcher {
	return txsFetcher{make(chan []*pb.Transaction, 1)}
}

// TxStore 公链监听到的交易以及网关本身交易的内存池
type TxStore struct {
	TxOverdueEvent *util.Event
	db             *dgwdb.LDBDatabase
	txInfoById     map[string]*txWithTimeMs  //未打包进区块的交易
	watchedTxInfo  map[string]*WatchedTxInfo //所有未处理完的主/侧链交易

	//新增的主/侧链交易，主节点一次批量获取到新增，然后转发给从节点
	freshWatchedTxInfo map[string]*WatchedTxInfo
	pendingFetchers    []txsFetcher
	currTerm           int64

	newTermChan      chan int64
	newCommitChan    chan blockCommitted
	fetchTxsChan     chan txsFetcher
	addTxsChan       chan addTxsRequest
	addWatchedTxChan chan *pb.WatchedTxInfo
	addFreshChan     chan *pb.WatchedTxInfo
	//queryTxsChan   chan txsQuery
	heartbeatTimer *time.Timer
	sync.RWMutex
}

// NewTxStore 新建一个TxStore对象并返回
func NewTxStore(db *dgwdb.LDBDatabase) *TxStore {
	txStore := &TxStore{
		TxOverdueEvent:     util.NewEvent(),
		db:                 db,
		txInfoById:         make(map[string]*txWithTimeMs),
		watchedTxInfo:      make(map[string]*WatchedTxInfo),
		freshWatchedTxInfo: make(map[string]*WatchedTxInfo),
		pendingFetchers:    nil,
		currTerm:           0,

		newTermChan:      make(chan int64),
		newCommitChan:    make(chan blockCommitted),
		addTxsChan:       make(chan addTxsRequest),
		fetchTxsChan:     make(chan txsFetcher),
		addWatchedTxChan: make(chan *pb.WatchedTxInfo),
		addFreshChan:     make(chan *pb.WatchedTxInfo),
		//queryTxsChan:   make(chan txsQuery),
		heartbeatTimer: time.NewTimer(heartbeatInterval),
	}

	return txStore
}

// Run TxStore的循环处理函数
func (ts *TxStore) Run(ctx context.Context) {
	go ts.heartbeatCheck(ctx)
	for {
		select {
		case addTxsReq := <-ts.addTxsChan:
			addTxsReq.notAddedChan <- ts.addTxs(addTxsReq.txs)
		case newCommit := <-ts.newCommitChan:
			ts.cleanUpOnNewCommitted(newCommit.txs, newCommit.height)
		case watchedTx := <-ts.addWatchedTxChan:
			bsLogger.Debug("add watched tx to mempool", "scTxID", watchedTx.Txid)
			ts.Lock()

			wtx := &WatchedTxInfo{
				Tx:                 watchedTx,
				genTs:              time.Now(),
				calcOverdueTs:      time.Now(),
				txWaitingTolerance: defaultTxWaitingTolerance,
			}
			//sign时去主链获取的主链交易不需要重新加入
			if _, exist := ts.watchedTxInfo[watchedTx.Txid]; !exist {
				ts.watchedTxInfo[watchedTx.Txid] = wtx
				// 多签资产分配交易不用放到fresh队列里面，由rpc接口手动触发
				if !watchedTx.IsDistributionTx() {
					ts.freshWatchedTxInfo[watchedTx.Txid] = wtx
				}
			}
			ts.Unlock()
		case watchedTx := <-ts.addFreshChan:

			wtx := &WatchedTxInfo{
				Tx:                 watchedTx,
				genTs:              time.Now(),
				calcOverdueTs:      time.Now(),
				txWaitingTolerance: defaultTxWaitingTolerance,
			}
			ts.Lock()
			bsLogger.Debug("add fresh tx to freshlist", "scTxID", watchedTx.Txid)
			ts.freshWatchedTxInfo[watchedTx.Txid] = wtx
			ts.Unlock()
		case term := <-ts.newTermChan:
			ts.currTerm = term
		case <-ctx.Done():
			return
		}
	}
}

// HasWatchedTx 是否已经接收过tx了，是返回true，否返回false
func (ts *TxStore) HasWatchedTx(tx *pb.WatchedTxInfo) bool {
	if tx != nil && tx.Txid != "" {
		ts.RLock()
		defer ts.RUnlock()
		_, inWatched := ts.watchedTxInfo[tx.Txid]
		inDB := ts.HasTxInDB(tx.Txid)
		return inWatched && inDB
	}
	return false
}

// OnNewBlockCommitted 新区块共识后的回调处理，需要清理内存池
func (ts *TxStore) OnNewBlockCommitted(newBlock *pb.BlockPack) {
	bsLogger.Debug("begin OnNewBlockCommitted", "blocktype", newBlock.Block().Type)
	if newBlock.IsTxsBlock() {
		req := makeBlockCommitted(newBlock)
		ts.newCommitChan <- req
	}
}

// OnTermChanged term更新时的处理
func (ts *TxStore) OnTermChanged(term int64) {
	// ts.newTermChan <- term
	bsLogger.Debug("set curr term", "term", term)
	ts.currTerm = term
}

// TestAddTxs just fot test api
func (ts *TxStore) TestAddTxs(txs []*pb.Transaction) []int {
	req := makeAddTxsRequest(txs)
	ts.addTxsChan <- req
	return <-req.notAddedChan
}

// AddWatchedTx 添加监听到的公链交易到内存池
func (ts *TxStore) AddWatchedTx(tx *pb.WatchedTxInfo) {
	ts.addWatchedTxChan <- tx
}

// AddFreshWatchedTx 增加新监听到的交易到待处理列表
func (ts *TxStore) AddFreshWatchedTx(tx *pb.WatchedTxInfo) {
	ts.RLock()
	inMem := ts.hasTxInMemPool(tx.Txid)
	ts.RUnlock()
	if tx != nil && !ts.HasTxInDB(tx.Txid) && !inMem { //未同步才添加到fresh
		go func() {
			ts.addFreshChan <- tx
		}()
	} else {
		bsLogger.Debug("tx has been in db or mem", "scTxID", tx.Txid)
	}
}

// GetFreshWatchedTxs 获取尚未被处理的公链交易
func (ts *TxStore) GetFreshWatchedTxs() []*WatchedTxInfo {
	ts.Lock()
	var rst []*WatchedTxInfo
	var reAppend []*WatchedTxInfo
	for _, v := range ts.freshWatchedTxInfo {
		// 本term内已经确定加签失败的交易，下个term再重新发起
		if IsSignFailed(ts.db, v.Tx.Txid, ts.currTerm) {
			reAppend = append(reAppend, v)
			continue
		}
		if ts.HasTxInDB(v.Tx.Txid) || ts.hasTxInMemPool(v.Tx.Txid) { //已从其他节点同步
			bsLogger.Debug("tx in fresh has been in mem or db", "scTxID", v.Tx.Txid)
			delete(ts.freshWatchedTxInfo, v.Tx.Txid)
			continue
		}
		bsLogger.Debug("add watched tx to queue", "sctxid", v.Tx.Txid)
		rst = append(rst, v)
	}
	ts.freshWatchedTxInfo = make(map[string]*WatchedTxInfo)
	for _, v := range reAppend {
		ts.freshWatchedTxInfo[v.Tx.Txid] = v
	}
	ts.Unlock()
	return rst
}

// HasFreshWatchedTx 是否还有未处理的公链交易
func (ts *TxStore) HasFreshWatchedTx() bool {
	ts.Lock()
	has := len(ts.freshWatchedTxInfo) > 0
	ts.Unlock()
	return has
}

// DeleteFresh 把交易从待处理的列表中删除
func (ts *TxStore) DeleteFresh(txId string) {
	ts.Lock()
	delete(ts.freshWatchedTxInfo, txId)
	ts.Unlock()
}

// DeleteWatchedTx 把交易从内存池中删除, 只会发生在多签地址的迁移交易里面
func (ts *TxStore) DeleteWatchedTx(txId string) {
	ts.Lock()
	delete(ts.watchedTxInfo, txId)
	ts.Unlock()
}

// CreateInnerTx 创建一笔网关本身的交易
func (ts *TxStore) CreateInnerTx(newlyTxId string, signMsgId string, amount int64) {
	signMsg := GetSignMsg(ts.db, signMsgId)
	if signMsg == nil {
		bsLogger.Error("create inner tx failed, sign msg not found", "signmsgid", signMsgId,
			"newlyTxId", newlyTxId)
		return
	}
	bsLogger.Debug("create inner tx", "from", signMsg.WatchedTx.From, "to", signMsg.WatchedTx.To,
		"sctxid", signMsg.WatchedTx.Txid, "newliTxId", newlyTxId)
	if amount == 0 {
		amount = signMsg.WatchedTx.Amount
	}
	innerTx := &pb.Transaction{
		WatchedTx: signMsg.WatchedTx.Clone(),
		NewlyTxId: newlyTxId,
		Time:      time.Now().Unix(),
		Amount:    amount,
	}
	// 这个时候监听到的交易已经成功处理并上链了，先清理监听交易缓存
	ts.Lock()
	delete(ts.watchedTxInfo, signMsg.WatchedTx.Txid)
	delete(ts.freshWatchedTxInfo, signMsg.WatchedTx.Txid)
	ts.Unlock()
	innerTx.UpdateId()
	req := makeAddTxsRequest([]*pb.Transaction{innerTx})
	ts.addTxsChan <- req
}

// QueryTxInfoBySidechainId 根据公链的交易id查询对应到的网关交易信息
func (ts *TxStore) QueryTxInfoBySidechainId(scId string) *TxQueryResult {
	return ts.queryTxsInfo([]string{scId})[0]
}

func (ts *TxStore) FetchTxsChan() <-chan []*pb.Transaction {
	fetcher := makeTxsFetcher()
	ts.fetchTxsChan <- fetcher
	return fetcher.resultChan
}

// GetMemTxs 获取内存池的网关交易
func (ts *TxStore) GetMemTxs() []*pb.Transaction {
	var fetched []*pb.Transaction
	ts.RLock()
	for _, v := range ts.txInfoById {
		fetched = append(fetched, v.tx)
	}
	ts.RUnlock()
	return fetched
}

func (ts *TxStore) addTxs(txs []*pb.Transaction) []int {
	notAddedIds := make([]int, 0)
	ts.Lock()
	for idx, tx := range txs {
		if ts.txInfoById[tx.WatchedTx.Txid] == nil && GetTxLookupEntry(ts.db, tx.Id) == nil {
			tt := &txWithTimeMs{tx, time.Now(), time.Now(), defaultTxWaitingTolerance}
			ts.txInfoById[tx.WatchedTx.Txid] = tt
		} else {
			notAddedIds = append(notAddedIds, idx)
		}
	}
	ts.Unlock()
	return notAddedIds
}

func (ts *TxStore) HasTxInMemPool(scTxId string) bool {
	ts.RLock()
	defer ts.RUnlock()
	_, ok := ts.txInfoById[scTxId]
	return ok
}

func (ts *TxStore) hasTxInMemPool(scTxId string) bool {
	_, ok := ts.txInfoById[scTxId]
	return ok
}

func (ts *TxStore) HasTxInDB(scTxId string) bool {
	tmp := GetTxIdBySidechainTxId(ts.db, scTxId)
	return tmp != nil
}

// 把被打包进新区块的交易从内存池里面删掉
func (ts *TxStore) cleanUpOnNewCommitted(committedTxs []*pb.Transaction, height int64) {
	for idx, tx := range committedTxs {
		entry := &pb.TxLookupEntry{
			Height: height,
			Index:  int32(idx),
		}
		bsLogger.Debug("write tx to db and delete from mempool", "txid", tx.WatchedTx.Txid)
		ts.Lock()
		SetTxLookupEntry(ts.db, tx.Id, entry)
		//from链 tx_id 和网关tx_id的对应
		SetTxIdMap(ts.db, tx.WatchedTx.Txid, tx.Id)
		//to链和tx_id 和网关tx_id的对应
		SetTxIdMap(ts.db, tx.NewlyTxId, tx.Id)
		delete(ts.txInfoById, tx.WatchedTx.Txid)
		delete(ts.watchedTxInfo, tx.WatchedTx.Txid)
		delete(ts.freshWatchedTxInfo, tx.WatchedTx.Txid)
		ts.Unlock()
	}
}

func (ts *TxStore) queryTxsInfo(scTxIds []string) (rst []*TxQueryResult) {
	for _, scTxId := range scTxIds {
		ts.RLock()
		value, ok := ts.txInfoById[scTxId]
		ts.RUnlock()
		if ok {
			rst = append(rst, &TxQueryResult{
				Tx:     value.tx,
				Height: -1,
			})
		} else {
			txId := GetTxIdBySidechainTxId(ts.db, scTxId)
			if txId == nil {
				bsLogger.Debug("get tx id failed", "sideId", scTxId)
				rst = append(rst, nil)
				continue
			}
			entry := GetTxLookupEntry(ts.db, txId)
			if entry == nil {
				bsLogger.Debug("query transaction not found", "txId", txId)
				rst = append(rst, nil)
				continue
			}
			assert.True(entry.Height <= GetCommitHeight(ts.db))
			blockPack := GetCommitByHeight(ts.db, entry.Height)
			if blockPack == nil {
				bsLogger.Debug("query transaction block not found", "height", entry.Height)
				rst = append(rst, nil)
				continue
			}
			assert.True(int(entry.Index) < len(blockPack.Block().Txs) && entry.Index >= 0)
			tx := blockPack.Block().Txs[entry.Index]
			rst = append(rst, &TxQueryResult{
				Tx:      tx,
				Height:  entry.Height,
				BlockID: blockPack.Id(),
			})
		}
	}
	return
}

func (ts *TxStore) GetTx(txid string) *TxQueryResult {
	dgwTxID := GetTxIdBySidechainTxId(ts.db, txid) //首先通过公链tx_id查询网关tx_id
	if dgwTxID == nil {                            //如果获取不到使用网关tx_id查询
		bsLogger.Debug("get dgw from txid failed", "sideId", txid)
		data, err := hex.DecodeString(txid)
		if err != nil {
			return nil
		}
		dgwTxID = &crypto.Digest256{
			Data: data,
		}
	}
	entry := GetTxLookupEntry(ts.db, dgwTxID)
	if entry == nil {
		bsLogger.Debug("query transaction not found", "txId", txid)
		return nil
	}
	assert.True(entry.Height <= GetCommitHeight(ts.db))
	blockPack := GetCommitByHeight(ts.db, entry.Height)
	if blockPack == nil {
		bsLogger.Debug("query transaction block not found", "height", entry.Height)
		return nil
	}
	assert.True(int(entry.Index) < len(blockPack.Block().Txs) && entry.Index >= 0)
	tx := blockPack.Block().Txs[entry.Index]
	return &TxQueryResult{
		Tx:      tx,
		Height:  entry.Height,
		BlockID: blockPack.Id(),
	}
}

func (ts *TxStore) heartbeatCheck(ctx context.Context) {
	for {
		select {
		case <-ts.heartbeatTimer.C:
			ts.doHeartbeat()
			ts.heartbeatTimer.Reset(heartbeatInterval)
		case <-ctx.Done():
			return
		}
	}
}

func (ts *TxStore) doHeartbeat() {
	innerTxHasOverdue, watchedTxHasOverdue := false, false

	ts.Lock()
	for id, txInfo := range ts.freshWatchedTxInfo {
		if txInfo.isOverdue() {
			// 由于节点之间的watcher不完全同步，也有可能是监听到这个交易之前，主节点已经广播处理了这笔交易了，所以做一下校验
			if _, has := ts.txInfoById[id]; has {
				//delete(ts.watchedTxInfo, id)
				//delete(ts.freshWatchedTxInfo, id)
				continue
			}
			if GetTxIdBySidechainTxId(ts.db, id) != nil {
				delete(ts.watchedTxInfo, id)
				delete(ts.freshWatchedTxInfo, id)
				continue
			}
			bsLogger.Error("watched tx timeout", "sctxid", id)
			watchedTxHasOverdue = true
			// 重新设置超时时间 防止重复accuse
			txInfo.resetWaitingTolerance()
		}
	}

	for id, txInfo := range ts.txInfoById {
		if txInfo.IsOverdue() {
			if GetTxIdBySidechainTxId(ts.db, id) != nil {
				delete(ts.txInfoById, id)
				continue
			}
			bsLogger.Error("confirm tx timeout", "sctxid", id)
			innerTxHasOverdue = true
			txInfo.resetWaitingTolerance()
		}
	}
	ts.Unlock()

	if innerTxHasOverdue || watchedTxHasOverdue {
		ts.TxOverdueEvent.Emit(GetNodeTerm(ts.db))
	}
}

// ValidateTx 验证交易的合法性
func (ts *TxStore) ValidateTx(tx *pb.Transaction) int {
	ts.RLock()
	defer ts.RUnlock()
	memTx := ts.txInfoById[tx.WatchedTx.Txid]
	if memTx == nil {
		watchedTx := ts.watchedTxInfo[tx.WatchedTx.Txid]
		if watchedTx == nil {
			// 如果本节点没有监听记录，则返回mempool里面不存在
			// 很可能是新加入的节点，跳过本次区块，下一次init区块的时候会去自动同步最新区块
			bsLogger.Warn("validate tx not exist", "sctxid", tx.WatchedTx.Txid)
			return NotExist
		} else if !watchedTx.Tx.EqualTo(tx.WatchedTx) {
			bsLogger.Warn("validate watchedtx invalid", "tx", tx.WatchedTx, "memtx", watchedTx)
			return Invalid
		} else {
			// 与本机watched的消息一致，但本机还没有监听到公链上的交易结果
			// 返回CheckChain直接去公链上验证
			return CheckChain
		}
	} else {
		if memTx.tx.EqualTo(tx) {
			return Valid
		}
		// 证明本机监听到的消息与传过来的消息不一致，先accuse再说
		bsLogger.Warn("validate gateway tx invalid", "sctxid", tx.WatchedTx.Txid)
		return Invalid
	}
}

// ValidateWatchedTx 验证leader传过来的公链交易是否和本节点一致
func (ts *TxStore) ValidateWatchedTx(tx *pb.WatchedTxInfo) int {
	ts.RLock()
	memTx := ts.watchedTxInfo[tx.Txid]
	ts.RUnlock()
	if memTx == nil {
		return NotExist
	}
	if tx.EqualTo(memTx.Tx) {
		return Valid
	}
	return Invalid
}
