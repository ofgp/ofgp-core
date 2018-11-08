package node

import (
	"bytes"
	"encoding/hex"
	"eosc/eoswatcher"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/wire"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	btcwatcher "github.com/ofgp/bitcoinWatcher/mortgagewatcher"
	"github.com/ofgp/ofgp-core/cluster"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/ofgp/ofgp-core/util/assert"
	context "golang.org/x/net/context"
)

var signTimeout int64 //单位s

const defaultSignSucTimeout = 15    //单位s
var checkSignInterval time.Duration //checkSign是否达成共识的周期

//统计sign suc的节点数
type SignedResultCache struct {
	cache       map[int32]*pb.SignedResult
	totalCount  int32
	signedCount int32
	errCnt      int32
	doneTime    time.Time
	initTime    int64
	sync.RWMutex
	doneFlag int32
}

func (cache *SignedResultCache) setDone() bool {
	return atomic.CompareAndSwapInt32(&cache.doneFlag, 0, 1)
}
func (cache *SignedResultCache) isDone() bool {
	return atomic.LoadInt32(&cache.doneFlag) == 1
}

func (cache *SignedResultCache) addTotalCount() {
	atomic.AddInt32(&cache.totalCount, 1)
}
func (cache *SignedResultCache) addSignedCount() {
	atomic.AddInt32(&cache.signedCount, 1)
}
func (cache *SignedResultCache) addErrCnt() {
	atomic.AddInt32(&cache.errCnt, 1)
}
func (cache *SignedResultCache) addCache(nodeID int32, signRes *pb.SignedResult) {
	cache.Lock()
	defer cache.Unlock()
	cache.cache[nodeID] = signRes
}
func (cache *SignedResultCache) getCache(nodeID int32) (*pb.SignedResult, bool) {
	cache.RLock()
	defer cache.RUnlock()
	res, ok := cache.cache[nodeID]
	return res, ok
}

// getTotalCount 获取收到签名结果的个数
func (cache *SignedResultCache) getTotalCount() int32 {
	return atomic.LoadInt32(&cache.totalCount)
}

// getSignedCount 获取收到签名成功的个数
func (cache *SignedResultCache) getSignedCount() int32 {
	return atomic.LoadInt32(&cache.signedCount)
}

func (cache *SignedResultCache) getErrCnt() int32 {
	return atomic.LoadInt32(&cache.errCnt)
}

func (node *BraftNode) clearOnFail(signReq *pb.SignTxRequest) {
	leaderLogger.Debug("clear on fail", "sctxid", signReq.WatchedTx.Txid)
	node.blockStore.MarkFailedSignRecord(signReq.WatchedTx.Txid, signReq.Term)

	node.signedResultCache.Delete(signReq.WatchedTx.Txid)

	if !signReq.WatchedTx.IsTransferTx() && !node.hasTxInWaitting(signReq.WatchedTx.Txid) {
		node.blockStore.DeleteSignReqMsg(signReq.WatchedTx.Txid)
		if signReq.WatchedTx.IsDistributionTx() {
			node.proposalManager.SetFailed(signReq.WatchedTx.Txid[14:])
		} else {
			leaderLogger.Debug("add to fresh queue", "sctxid", signReq.WatchedTx.Txid)
			node.txStore.AddFreshWatchedTx(signReq.WatchedTx)
		}
	} else {
		leaderLogger.Debug("just delete watched tx", "sctxid", signReq.WatchedTx.Txid, "is_in_waiting", node.hasTxInWaitting(signReq.WatchedTx.Txid))
		node.txStore.DeleteWatchedTx(signReq.WatchedTx.Txid)
	}
}

// sendEOSTxToChain send eos transaction to chain
func (node *BraftNode) sendEOSTxToChain(watcher eoswatcher.EOSWatcherInterface,
	sigs [][][]byte, req *pb.SignTxRequest, signRes *pb.SignedResult) (newTxID string, err error) {
	var tmpSigs []*ecc.Signature
	for _, sig := range sigs {
		s := &ecc.Signature{}
		s.UnmarshalJSON(sig[0])
		tmpSigs = append(tmpSigs, s)
	}
	pack := &eos.PackedTransaction{
		Compression:       0,
		PackedTransaction: req.NewlyTx.Data,
	}
	transfer, _ := pack.Unpack()
	newlyTx, err := watcher.MergeSignedTx(transfer, tmpSigs...)
	if err != nil {
		leaderLogger.Error("merge sign tx failed", "err", err, "sctxid", signRes.TxId)
		node.clearOnFail(req)
		err = errors.New("erge sign tx failed")
		return
	}
	_, err = watcher.SendTx(newlyTx)
	if err != nil {
		leaderLogger.Error("send signed tx to eos failed", "err", err, "sctxid", signRes.TxId)
	}
	newlyTxHash := hex.EncodeToString(newlyTx.ID())
	return newlyTxHash, nil
}

func (node *BraftNode) sendBtcBchTxToChain(watcher *btcwatcher.MortgageWatcher, tx []byte,
	sigs [][][]byte, req *pb.SignTxRequest, signRes *pb.SignedResult) (newTxID string, err error) {
	buf := bytes.NewBuffer(tx)
	newlyTx := new(wire.MsgTx)
	err = newlyTx.Deserialize(buf)
	assert.ErrorIsNil(err)
	newTxID = newlyTx.TxHash().String()
	ok := watcher.MergeSignTx(newlyTx, sigs)
	if !ok {
		leaderLogger.Error("merge sign tx failed", "sctxid", signRes.TxId)
		node.clearOnFail(req)
		return
	}
	var sendedTxHash string
	sendedTxHash, err = watcher.SendTx(newlyTx)
	leaderLogger.Debug("sendedtx", "txid", sendedTxHash, "sctxid", signRes.TxId)
	return
}

func (node *BraftNode) sendTxToChain(chain string, tx []byte, sigs [][][]byte, signResult *pb.SignedResult, signReq *pb.SignTxRequest) {
	// if chain == "btc" || chain == "bch" {
	// 	var watcher *btcwatcher.MortgageWatcher
	// 	buf := bytes.NewBuffer(tx)
	// 	newlyTx := new(wire.MsgTx)
	// 	err := newlyTx.Deserialize(buf)
	// 	assert.ErrorIsNil(err)

	// 	if chain == "bch" {
	// 		watcher = node.bchWatcher
	// 	} else {
	// 		watcher = node.btcWatcher
	// 	}

	// 	newlyTxHash := newlyTx.TxHash().String()
	// 	ok := watcher.MergeSignTx(newlyTx, sigs)
	// 	if !ok {
	// 		leaderLogger.Error("merge sign tx failed", "sctxid", signResult.TxId)
	// 		node.clearOnFail(signReq)
	// 		return
	// 	}
	// 	start := time.Now().UnixNano()
	// 	_, err = watcher.SendTx(newlyTx)
	// 	end := time.Now().UnixNano()
	// 	leaderLogger.Debug("sendBchtime", "time", (end-start)/1e6)
	// 	if err != nil {
	// 		leaderLogger.Error("send signed tx to bch failed", "err", err, "sctxid", signResult.TxId)
	// 	}
	// 	node.blockStore.SignedTxEvent.Emit(newlyTxHash, signResult.TxId, signResult.To, signReq.WatchedTx.TokenTo)
	switch chain {
	case "btc":
		start := time.Now().UnixNano()
		newlyTxHash, err := node.sendBtcBchTxToChain(node.btcWatcher, tx, sigs, signReq, signResult)
		end := time.Now().UnixNano()
		leaderLogger.Debug("sendbtctime", "time", (end-start)/1e6)
		if err != nil {
			leaderLogger.Error("send signed tx to btc failed", "err", err, "sctxid", signResult.TxId)
		}
		node.blockStore.SignedTxEvent.Emit(newlyTxHash, signResult.TxId, signResult.To, signReq.WatchedTx.TokenTo)
	case "bch":
		start := time.Now().UnixNano()
		newlyTxHash, err := node.sendBtcBchTxToChain(node.bchWatcher, tx, sigs, signReq, signResult)
		end := time.Now().UnixNano()
		leaderLogger.Debug("sendbchtime", "time", (end-start)/1e6)
		if err != nil {
			leaderLogger.Error("send signed tx to btc failed", "err", err, "sctxid", signResult.TxId)
		}
		node.blockStore.SignedTxEvent.Emit(newlyTxHash, signResult.TxId, signResult.To, signReq.WatchedTx.TokenTo)
	case "xin":
		// var tmpSigs []*ecc.Signature
		// for _, sig := range sigs {
		// 	s := &ecc.Signature{}
		// 	s.UnmarshalJSON(sig[0])
		// 	tmpSigs = append(tmpSigs, s)
		// }
		// pack := &eos.PackedTransaction{
		// 	Compression:       0,
		// 	PackedTransaction: signReq.NewlyTx.Data,
		// }
		// transfer, _ := pack.Unpack()
		// newlyTx, err := node.xinWatcher.MergeSignedTx(transfer, tmpSigs...)
		// if err != nil {
		// 	leaderLogger.Error("merge sign tx failed", "sctxid", signResult.TxId)
		// 	node.clearOnFail(signReq)
		// 	return
		// }
		// _, err = node.xinWatcher.SendTx(newlyTx)
		// if err != nil {
		// 	leaderLogger.Error("send signed tx to xin failed", "err", err, "sctxid", signResult.TxId)
		// }
		// newlyTxHash := hex.EncodeToString(newlyTx.ID())
		newlyTxHash, _ := node.sendEOSTxToChain(node.xinWatcher, sigs, signReq, signResult)
		node.blockStore.SignedTxEvent.Emit(newlyTxHash, signResult.TxId, signResult.To, signReq.WatchedTx.TokenTo)
	case "eos":
		newlyTxHash, _ := node.sendEOSTxToChain(node.eosWatcher, sigs, signReq, signResult)
		node.blockStore.SignedTxEvent.Emit(newlyTxHash, signResult.TxId, signResult.To, signReq.WatchedTx.TokenTo)
	}
}

/*
func (node *BraftNode) sendTxToChain(newlyTx *wire.MsgTx, watcher *btcwatcher.MortgageWatcher,
	sigs [][][]byte, signResult *pb.SignedResult, signReq *pb.SignTxRequest) {

	// mergesigs的顺序需要与生成多签地址的顺序严格一致，所以按nodeid来顺序添加返回的sig
	newlyTxHash := newlyTx.TxHash().String()
	ok := watcher.MergeSignTx(newlyTx, sigs)
	if !ok {
		leaderLogger.Error("merge sign tx failed", "sctxid", signResult.TxId)
		node.clearOnFail(signReq)
		return
	}
	start := time.Now().UnixNano()
	_, err := watcher.SendTx(newlyTx)
	end := time.Now().UnixNano()
	leaderLogger.Debug("sendBchtime", "time", (end-start)/1e6)
	if err != nil {
		leaderLogger.Error("send signed tx to bch failed", "err", err, "sctxid", signResult.TxId)
	}
	node.blockStore.SignedTxEvent.Emit(newlyTxHash, signResult.TxId, signResult.To, signReq.WatchedTx.TokenTo)
}
*/

func (node *BraftNode) doSave(msg *pb.SignedResult) {
	leaderLogger.Debug("receive signres", "sctxid", msg.TxId)
	if node.blockStore.IsSignFailed(msg.TxId, msg.Term) {
		leaderLogger.Debug("signmsg is failed in this term", "sctxid", msg.TxId, "term", msg.Term)
		return
	}
	signReqMsg := node.blockStore.GetSignReqMsg(msg.TxId)
	cacheTemp, loaded := node.signedResultCache.LoadOrStore(msg.TxId, &SignedResultCache{
		cache:       make(map[int32]*pb.SignedResult),
		totalCount:  0,
		signedCount: 0,
		initTime:    time.Now().Unix(),
	})

	cache, ok := cacheTemp.(*SignedResultCache)
	if !ok {
		return
	}
	//如果不是第一次添加
	if loaded {
		if _, exist := cache.getCache(msg.NodeId); exist {
			leaderLogger.Debug("already receive signedres", "nodeID", msg.NodeId, "scTxID", msg.TxId)
			return
		}
	}
	cache.addTotalCount()
	// 由于网络延迟，有可能先收到了其他节点的签名结果，后收到签名请求，这个时候只做好保存即可
	if signReqMsg == nil {
		if msg.Code == pb.CodeType_SIGNED {
			cache.addSignedCount()
			cache.addCache(msg.NodeId, msg)
		} else {
			cache.addErrCnt()
		}
		return
	}

	if msg.Code == pb.CodeType_SIGNED {
		if !node.verifySign(msg.To, signReqMsg.NewlyTx.Data, msg.Data, msg.NodeId) {
			leaderLogger.Error("verify sign tx failed", "from", msg.NodeId, "sctxid", msg.TxId)
			cache.addErrCnt()
		} else {
			cache.addCache(msg.NodeId, msg)
			cache.addSignedCount()
		}
	} else {
		cache.addErrCnt()
	}

	var (
		quorumN       int32
		accuseQuorumN int32
	)
	if signReqMsg.WatchedTx.IsTransferTx() {
		snapshot := node.blockStore.GetClusterSnapshot(signReqMsg.MultisigAddress)
		if snapshot == nil {
			leaderLogger.Error("receive invalid transfer tx", "txid", signReqMsg.WatchedTx.Txid)
			return
		}
		quorumN = int32(snapshot.QuorumN)
		accuseQuorumN = int32(snapshot.AccuseQuorumN)
	} else {
		quorumN = int32(cluster.QuorumN)
		accuseQuorumN = int32(cluster.AccuseQuorumN)
	}

	if cache.getSignedCount() >= quorumN && !cache.isDone() && signReqMsg != nil {
		if cache.setDone() {
			cache.doneTime = time.Now()
			var sigs [][][]byte
			for idx := range cluster.NodeList {
				if result, has := cache.getCache(int32(idx)); has {
					leaderLogger.Debug("will merge sign info", "from", idx)
					sigs = append(sigs, result.Data)
					if len(sigs) == int(quorumN) {
						break
					}
				}
			}
			// sendTxToChain的时间可能会比较长，因为涉及到链上交易，所以需要提前把锁释放
			node.sendTxToChain(msg.To, signReqMsg.NewlyTx.Data, sigs, msg, signReqMsg)
		}
	} else if cache.getErrCnt() > accuseQuorumN {
		// 本次交易确认失败，清理缓存的数据，避免干扰后续的重试
		leaderLogger.Debug("sign accuseQuorumN fail")
		node.clearOnFail(signReqMsg)
	}
}

func (node *BraftNode) saveSignedResult(ctx context.Context) {
	//定期删除已处理的sign
	clearCh := time.NewTicker(3 * cluster.BlockInterval).C
	for {
		select {
		case msg := <-node.signedResultChan:
			go node.doSave(msg)
		case <-clearCh:
			node.signedResultCache.Range(func(k, v interface{}) bool {
				txID := k.(string)
				cache := v.(*SignedResultCache)
				if cache.isDone() && time.Now().After(cache.doneTime.Add(cacheTimeout)) {
					node.signedResultCache.Delete(txID)
				}
				return true
			})
		case <-ctx.Done():
			return
		}
	}
}

func (node *BraftNode) verifySign(chain string, tx []byte, sig [][]byte, nodeID int32) bool {
	if chain == "bch" || chain == "btc" {
		var watcher *btcwatcher.MortgageWatcher
		buf := bytes.NewBuffer(tx)
		newlyTx := new(wire.MsgTx)
		err := newlyTx.Deserialize(buf)
		if err != nil {
			return false
		}
		if chain == "bch" {
			watcher = node.bchWatcher
		} else {
			watcher = node.btcWatcher
		}
		if !watcher.VerifySign(newlyTx, sig, cluster.NodeList[nodeID].PublicKey) {
			leaderLogger.Error("verify sign tx failed", "from", nodeID)
			return false
		}
		return true
	} else if chain == "xin" {
		if len(tx) == 0 {
			leaderLogger.Error("verify xin signtx tx is nil", "from", nodeID)
			return false
		}
		pack := &eos.PackedTransaction{
			Compression:       0,
			PackedTransaction: tx,
		}
		transfer, err := pack.Unpack()
		if err != nil {
			leaderLogger.Error("verify xin signtx unpack failed", "from", nodeID, "err", err)
			return false
		}
		xinSig := &ecc.Signature{}
		err = xinSig.UnmarshalJSON(sig[0])
		if err != nil {
			leaderLogger.Error("verify xin signtx  Unmarshaljson failed", "from", nodeID, "err", err)
			return false
		}
		pubkey, err := node.xinWatcher.GetPublickeyFromTx(transfer, xinSig)
		if err != nil {
			leaderLogger.Error("verify xin signtx getpubkey failed", "from", nodeID, "err", err)
			return false
		}
		nodePubkey, err := node.xinWatcher.NewPublicKey(hex.EncodeToString(cluster.NodeList[nodeID].PublicKey))
		if err != nil {
			leaderLogger.Error("verify xin signtx getpubkeyfromnode failed", nodeID, "err", err)
		}
		isEqual := pubkey.String() == nodePubkey.String()
		if !isEqual {
			leaderLogger.Error("pubkey not equal", "txpubkey", pubkey.String(), "nodepubkey", nodePubkey.String())
		}
		return isEqual
	} else if chain == "eos" {
		pack := &eos.PackedTransaction{
			Compression:       0,
			PackedTransaction: tx,
		}
		transfer, err := pack.Unpack()
		if err != nil {
			leaderLogger.Error("verify eos signtx upack err", "from", nodeID, "err", err)
			return false
		}
		xinSig := &ecc.Signature{}
		err = xinSig.UnmarshalJSON(sig[0])
		if err != nil {
			leaderLogger.Error("verify eos signtx UnmarshalJson err", "from", nodeID, "err", err)
			return false
		}
		pubkey, err := node.eosWatcher.GetPublickeyFromTx(transfer, xinSig)
		if err != nil {
			leaderLogger.Error("verify eos signtx getPubkey err", "from", nodeID, "err", err)
			return false
		}
		nodePubkey, _ := node.eosWatcher.NewPublicKey(hex.EncodeToString(cluster.NodeList[nodeID].PublicKey))
		isEqual := pubkey.String() == nodePubkey.String()
		if !isEqual {
			leaderLogger.Error("pubkey not equal", "txpubkey", pubkey.String(), "nodepubkey", nodePubkey.String())
		}
		return isEqual
	}
	return false
}

func (node *BraftNode) hasTxInWaitting(scTxID string) bool {
	node.mu.Lock()
	defer node.mu.Unlock()
	_, ok := node.waitingConfirmTxs[scTxID]
	return ok
}

func (node *BraftNode) checkSignTimeout() {
	node.signedResultCache.Range(func(k, v interface{}) bool {
		scTxID := k.(string)
		cache := v.(*SignedResultCache)
		now := time.Now().Unix()
		if now-cache.initTime > signTimeout && !cache.isDone() { //sign达成共识超时，重新放回处理队列

			leaderLogger.Debug("sign timeout", "scTxID", scTxID)

			//删除sign标记
			node.signedResultCache.Delete(scTxID)
			signReq := node.blockStore.GetSignReqMsg(scTxID)
			if signReq == nil { //本地尚未签名
				return true
			}

			if !signReq.WatchedTx.IsTransferTx() {
				if !node.hasTxInWaitting(scTxID) { //如果签名已经共识
					node.blockStore.DeleteSignReqMsg(scTxID)
					node.txStore.AddFreshWatchedTx(signReq.WatchedTx)
				} else {
					nodeLogger.Debug("tx is in waiting", "scTxID", scTxID)
				}
			} else {
				node.blockStore.DeleteSignReqMsg(scTxID)
				node.txStore.DeleteWatchedTx(scTxID)
			}
		}
		return true

	})
}

func (node *BraftNode) runCheckSignTimeout(ctx context.Context) {
	go func() {
		if checkSignInterval == 0 {
			checkSignInterval = cluster.BlockInterval
		}
		if signTimeout == 0 {
			signTimeout = defaultSignSucTimeout
		}
		tch := time.NewTicker(checkSignInterval).C
		for {
			select {
			case <-tch:
				node.checkSignTimeout()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (node *BraftNode) SaveSignedResult(msg *pb.SignedResult) {
	node.signedResultChan <- msg
}
