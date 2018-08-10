package node

import (
	"bytes"
	"strconv"
	"sync"
	"time"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/log"
	"github.com/ofgp/ofgp-core/primitives"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/ofgp/ofgp-core/util"
	"github.com/ofgp/ofgp-core/util/assert"

	"github.com/spf13/viper"
	context "golang.org/x/net/context"

	btcwatcher "github.com/ofgp/bitcoinWatcher/mortgagewatcher"

	ew "github.com/ofgp/ethwatcher"
)

const (
	votePoolTermRange = 500
	cacheTimeout      = 15 * time.Second
)

var (
	leaderLogger = log.New(viper.GetString("loglevel"), "leader")
)

// Leader leader节点描述
type Leader struct {
	BecomeLeaderEvent *util.Event
	beComeLeaderCnt   int32 //成为leader的次数
	NewInitEvent      *util.Event
	RetireEvent       *util.Event

	nodeInfo cluster.NodeInfo

	term          int64                //current term
	votes         pb.VoteMap           //votes for the current term with usable votie
	initing       *pb.InitMsg          //leading the cluster to commit this block
	votePool      map[int64]pb.VoteMap //a pool of received votes, including votes for future terms
	newNodeHost   string
	leavingNodeId int32
	hasTxToSign   bool

	newTermChan      chan int64
	newCommittedChan chan *pb.BlockPack
	voteChan         chan *pb.Vote
	nodeHostChan     chan string
	leaveNodeChan    chan int32

	blockStore *primitives.BlockStore
	txStore    *primitives.TxStore
	signer     *crypto.SecureSigner
	bchWatcher *btcwatcher.MortgageWatcher
	btcWatcher *btcwatcher.MortgageWatcher
	ethWatcher *ew.Client
	pm         *cluster.PeerManager
	sync.Mutex
}

// NewLeader 新生成一个leader对象，并启动后台任务，循环检查选举相关任务（创建块，投票等）
func NewLeader(nodeInfo cluster.NodeInfo, bs *primitives.BlockStore, ts *primitives.TxStore,
	signer *crypto.SecureSigner, btcWatcher *btcwatcher.MortgageWatcher, bchWatcher *btcwatcher.MortgageWatcher,
	ethWatcher *ew.Client, pm *cluster.PeerManager) *Leader {
	leader := &Leader{
		BecomeLeaderEvent: util.NewEvent(),
		NewInitEvent:      util.NewEvent(),
		RetireEvent:       util.NewEvent(),

		leavingNodeId: -1,
		nodeInfo:      nodeInfo,
		hasTxToSign:   false,

		term:     bs.GetNodeTerm(),
		votes:    make(pb.VoteMap),
		initing:  nil,
		votePool: make(map[int64]pb.VoteMap),

		newTermChan:      make(chan int64),
		newCommittedChan: make(chan *pb.BlockPack),
		voteChan:         make(chan *pb.Vote),
		nodeHostChan:     make(chan string),
		leaveNodeChan:    make(chan int32),

		blockStore: bs,
		txStore:    ts,
		signer:     signer,
		bchWatcher: bchWatcher,
		btcWatcher: btcWatcher,
		ethWatcher: ethWatcher,
		pm:         pm,
	}

	bs.NewTermEvent.Subscribe(func(newTerm int64) {
		leader.newTermChan <- newTerm
	})
	bs.NewCommittedEvent.Subscribe(func(newTop *pb.BlockPack) {
		leader.newCommittedChan <- newTop
	})

	return leader
}

// AddVote 处理收到的投票
func (ld *Leader) AddVote(vote *pb.Vote) {
	ld.voteChan <- vote
}

func (ld *Leader) OnNewNodeJoin(host string) {
	ld.nodeHostChan <- host
}

func (ld *Leader) OnNodeJoinedDone(vote *pb.Vote) {
	// 我们假定一次只能添加一个节点，所以这里没有用锁，如果需要的话，可以对voteChan做一些调整
	ld.justAddVote(vote)
	ld.nodeHostChan <- ""
}

func (ld *Leader) OnJoinCancel() {
	ld.nodeHostChan <- ""
}

func (ld *Leader) OnNodeLeave(nodeId int32) {
	ld.leaveNodeChan <- nodeId
}

func (ld *Leader) OnNodeLeaveDone() {
	ld.leaveNodeChan <- -1
}

func (ld *Leader) Run(ctx context.Context) {
	go ld.createTransaction(ctx)
	go ld.watchFormerMultisig(ctx)
	tick := time.Tick(cluster.BlockInterval)
	for {
		select {
		case <-tick:
			nodeLogger.Debug("tick...")
			if ld.readyToInitNewBlock() {
				nodeLogger.Debug("leader ready to init new block", "term", ld.term)
				txs := ld.txStore.GetMemTxs()
				if len(txs) > 0 {
					ld.tryCreateBlock(txs)
				} else {
					ld.tryCreateBlock(nil)
				}
			}
		case newTerm := <-ld.newTermChan:
			ld.updateTerm(newTerm)
		case committed := <-ld.newCommittedChan:
			if ld.isInCharge() {
				if ld.initing != nil && committed.BlockId().EqualTo(ld.initing.BlockId()) {
					ld.initing = nil //ready to create next block
				}
			} else {
				ld.scanVotePool()
			}
		case vote := <-ld.voteChan:
			ld.addVote(vote)
		case host := <-ld.nodeHostChan:
			ld.newNodeHost = host
		case nodeId := <-ld.leaveNodeChan:
			ld.leavingNodeId = nodeId
		case <-ctx.Done():
			return
		}
	}
}

func (ld *Leader) isInCharge() bool {
	return cluster.LeaderNodeOfTerm(ld.term) == ld.nodeInfo.Id &&
		len(ld.votes) >= cluster.QuorumN
}

func (ld *Leader) readyToInitNewBlock() bool {
	return ld.isInCharge() && ld.initing == nil
}

func (ld *Leader) updateTerm(newTerm int64) {
	if newTerm <= ld.term {
		return
	}

	wasInCharge := ld.isInCharge()

	ld.term = newTerm
	ld.votes = make(map[int32]*pb.Vote)
	ld.initing = nil

	ld.scanVotePool()

	if wasInCharge && !ld.isInCharge() {
		ld.RetireEvent.Emit(ld.nodeInfo, newTerm)
	}
}

func (ld *Leader) addVote(vote *pb.Vote) {
	if ld.term <= vote.Term && vote.Term <= ld.term+votePoolTermRange {
		if ld.votePool[vote.Term] == nil {
			ld.votePool[vote.Term] = make(pb.VoteMap)
		}
		ld.votePool[vote.Term][vote.NodeId] = vote
		ld.scanVotePool()
	}
}

// 1. 删除之前的term的votes
// 2. 把pool里面可用的votes放到{ld.votes}
// 3. 检查ld是否变成了主节点
func (ld *Leader) scanVotePool() {
	for term := range ld.votePool {
		if term < ld.term {
			delete(ld.votePool, term)
		}
	}

	if ld.isInCharge() {
		return
	}

	commitTop := ld.blockStore.GetCommitTop()
	for nodeId, vote := range ld.votePool[ld.term] {
		if _, has := ld.votes[nodeId]; has {
			continue
		}

		if vote.Votie.HasLessTermHeightThan(commitTop.ToVotie()) ||
			vote.Votie.Block.Id.EqualTo(commitTop.BlockId()) ||
			vote.Votie.Block.PrevBlockId.EqualTo(commitTop.BlockId()) {
			ld.votes[nodeId] = vote
		}
	}

	if ld.isInCharge() {
		ld.BecomeLeaderEvent.Emit(ld.nodeInfo, ld.term)
		assert.True(ld.initing == nil)
		//ld.tryCreateBlock(nil, nil)
		ld.tryCreateBlock(nil)
	}
}

// 适用于新节点加入，默认给节点投票，不需要验证
func (ld *Leader) justAddVote(vote *pb.Vote) {
	vote.Term = ld.term
	if ld.votePool[vote.Term] == nil {
		ld.votePool[vote.Term] = make(pb.VoteMap)
	}
	ld.votePool[vote.Term][vote.NodeId] = vote

	if _, has := ld.votes[vote.NodeId]; has {
		return
	}
	commitTop := ld.blockStore.GetCommitTop()
	if vote.Votie.HasLessTermHeightThan(commitTop.ToVotie()) ||
		vote.Votie.Block.Id.EqualTo(commitTop.BlockId()) ||
		vote.Votie.Block.PrevBlockId.EqualTo(commitTop.BlockId()) {
		ld.votes[vote.NodeId] = vote
	}
}

func (ld *Leader) tryCreateBlock(txs []*pb.Transaction) {
	assert.True(ld.readyToInitNewBlock())
	ld.tryCreateBlockImpl(txs)
}

func (ld *Leader) tryCreateBlockImpl(txs []*pb.Transaction) {
	nodeLogger.Debug("leader try creating block")
	if ld.blockStore.GetNodeTerm() != ld.term {
		leaderLogger.Error("the term is changed when trying to create a new block", "ld.term", ld.term, "nodeTerm", ld.blockStore.GetNodeTerm())
		return
	}
	if ld.blockStore.GetFresh() != nil {
		leaderLogger.Error("leader has a fresh when trying to create a new block")
		return
	}

	top := ld.blockStore.GetCommitTop()
	maxVotie := ld.votes.GetMaxVotie()

	var blockToInit *pb.Block

	// if maxVotie is connecting the current top, re-init the block in maxVotie;
	// otherwise, create and init a new block
	if top.ToVotie().HasLessTermHeightThan(maxVotie) && !maxVotie.Block.Id.EqualTo(top.BlockId()) {
		if !maxVotie.Block.PrevBlockId.EqualTo(top.BlockId()) {
			leaderLogger.Error("Max votie is neither committed nor connecting current top")
			return
		}
		leaderLogger.Debug("reinit the max-votie in a new term")
		blockToInit = maxVotie.Block
	} else {
		if len(ld.newNodeHost) > 0 {
			blockToInit = pb.CreateJoinReconfigBlock(util.NowMs(), top.BlockId(), pb.Reconfig_JOIN, ld.newNodeHost, int32(len(cluster.NodeList)))
		} else if ld.leavingNodeId >= 0 {
			blockToInit = pb.CreateLeaveReconfigBlock(util.NowMs(), top.BlockId(), pb.Reconfig_LEAVE, ld.leavingNodeId)
		} else {
			// 因为交易都是watcher监听到的，不存在外部创建交易，所以暂时先不做交易的合法性校验，在handleInit里面统一做
			blockToInit = pb.CreateTxsBlock(util.NowMs(), top.BlockId(), txs)
		}
	}

	if blockToInit == nil {
		return
	}

	init := &pb.InitMsg{
		Term:   ld.term,
		Height: top.Height() + 1,
		Block:  blockToInit,
		NodeId: ld.nodeInfo.Id,
	}
	// 如果是主节点第一次产生区块，需要提供votes来证明主节点的合法性
	if top.Term() < ld.term || init.PrevBlockId().EqualTo(maxVotie.Block.Id) {
		init.Votes = ld.votes
	}

	sig, err := ld.signer.Sign(init.Id().Data)
	if err != nil {
		leaderLogger.Error("sign block failed", "err", err)
		return
	}
	init.Sig = sig
	ld.initing = init
	leaderLogger.Debug("create init block done")
	ld.NewInitEvent.Emit(init)
}

// 循环监听老的多签地址，如果老的多签地址有UTXO，则把他们转移到新的多签地址
func (ld *Leader) watchFormerMultisig(ctx context.Context) {
	// tick := time.Tick(1 * time.Hour)
	tick := time.Tick(3 * time.Minute)
	for {
		select {
		case <-tick:
			leaderLogger.Debug("begin watch former multisig")
			cluster.MultiSigSnapshot.Lock()
		JLoop:
			for _, multiSig := range cluster.MultiSigSnapshot.SigInfos {
				if ld.isInCharge() {
					leaderLogger.Debug("watcher former multisig", "bchaddress", multiSig.BchAddress, "btcaddress", multiSig.BtcAddress)
					addressMap := make(map[string]string)
					addressMap["bch"] = multiSig.BchAddress
					addressMap["btc"] = multiSig.BtcAddress
					for chainType, address := range addressMap {
						var watcher *btcwatcher.MortgageWatcher
						if chainType == "btc" {
							watcher = ld.btcWatcher
						} else {
							watcher = ld.bchWatcher
						}
						utxoList := watcher.GetUnspentUtxo(address)
						for {
							if len(utxoList) == 0 {
								break
							}
							leaderLogger.Debug("multisig has unspend utxo", "address", address, "utxolen", len(utxoList))
							watchedTxInfo := &pb.WatchedTxInfo{
								Txid: "TransferTx" + strconv.FormatInt(util.NowMs(), 10),
								From: chainType,
								To:   chainType,
							}
							clusterSnapshot := ld.blockStore.GetClusterSnapshot(address)
							transferTx := ld.createTransferTx(watcher, address, clusterSnapshot)
							signTxReq, err := pb.MakeSignTxMsg(ld.blockStore.GetNodeTerm(), ld.nodeInfo.Id,
								watchedTxInfo, transferTx, address, ld.signer)
							if err != nil {
								leaderLogger.Error("make sign transfer tx failed", "err", err)
								continue
							}
							if !ld.isInCharge() {
								break JLoop
							}
							ld.broadcastSign(signTxReq, clusterSnapshot.NodeList, clusterSnapshot.QuorumN)
							utxoList = watcher.GetUnspentUtxo(address)
						}
					}
				} else {
					break
				}
			}
			cluster.MultiSigSnapshot.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

// 循环处理监听到的交易
func (ld *Leader) createTransaction(ctx context.Context) {
	tick := time.Tick(time.Duration(100) * time.Millisecond)
	for {
		select {
		case <-tick:
			if ld.isInCharge() {
				txs := ld.txStore.GetFreshWatchedTxs()
				if len(txs) > 0 {
					leaderLogger.Debug("get fresh tx from mempool", "len", len(txs))
					ld.hasTxToSign = true
				}
				for _, tx := range txs {
					var newlyTx *pb.NewlyTx
					if !ld.isInCharge() {
						ld.txStore.AddFreshWatchedTx(tx.Tx)
						continue
					}

					leaderLogger.Debug("begin sign watched tx", "sctxid", tx.Tx.Txid)
					if tx.Tx.To == "bch" {
						newlyTx = ld.createBtcTx(tx.Tx, "bch")
					} else if tx.Tx.To == "eth" {
						newlyTx = ld.createEthInput(tx.Tx)
					} else if tx.Tx.To == "btc" {
						newlyTx = ld.createBtcTx(tx.Tx, "btc")
					} else {
						leaderLogger.Error("watched tx wrong type", "type", tx.Tx.To)
						continue
					}
					if newlyTx == nil {
						//创建交易失败重新添加到fresh
						ld.txStore.AddFreshWatchedTx(tx.Tx)
						continue
					}
					signTxReq, err := pb.MakeSignTxMsg(ld.blockStore.GetNodeTerm(), ld.nodeInfo.Id,
						tx.Tx.Clone(), newlyTx, "", ld.signer)
					if err != nil {
						leaderLogger.Error("make sign tx failed", "err", err)
						continue
					}
					if !ld.isInCharge() {
						ld.txStore.AddFreshWatchedTx(tx.Tx)
						continue
					}
					ld.broadcastSign(signTxReq, cluster.NodeList, cluster.QuorumN)
					leaderLogger.Debug("broadcast sign done", "sctxid", tx.Tx.Txid)
				}
				ld.hasTxToSign = false
			}
		case <-ctx.Done():
			return
		}
	}
}

func (ld *Leader) createTransferTx(watcher *btcwatcher.MortgageWatcher, address string,
	snapshot *cluster.Snapshot) *pb.NewlyTx {
	leaderLogger.Debug("transfer param", "quorum", snapshot.QuorumN, "clusterSize", snapshot.ClusterSize)
	newlyTx := watcher.TransferAsset(address, snapshot.QuorumN, snapshot.ClusterSize)
	if newlyTx == nil {
		return nil
	}
	buf := bytes.NewBuffer([]byte{})
	err := newlyTx.Serialize(buf)
	if err != nil {
		leaderLogger.Error("serialize newly tx failed", "err", err)
		return nil
	}
	return &pb.NewlyTx{Data: buf.Bytes()}
}

func (ld *Leader) createBtcTx(watchedTx *pb.WatchedTxInfo, chainType string) *pb.NewlyTx {
	var watcherAddressInfo []*btcwatcher.AddressInfo
	var watcher *btcwatcher.MortgageWatcher
	if chainType == "bch" {
		watcher = ld.bchWatcher
	} else {
		watcher = ld.btcWatcher
	}

	for _, a := range watchedTx.RechargeList {
		watcherAddressInfo = append(watcherAddressInfo, &btcwatcher.AddressInfo{
			Amount:  a.Amount,
			Address: a.Address,
		})
	}
	leaderLogger.Debug("rechargelist", "sctxid", watchedTx.Txid, "addrs", watcherAddressInfo)
	newlyTx, ok := watcher.CreateCoinTx(watcherAddressInfo, watchedTx.Fee, watchedTx.Txid)
	if ok != 0 {
		leaderLogger.Error("create new chan tx failed", "errcode", ok, "sctxid", watchedTx.Txid)
		return nil
	}
	leaderLogger.Debug("create coin tx", "sctxid", watchedTx.Txid, "newlyTxid", newlyTx.TxHash().String())

	buf := bytes.NewBuffer([]byte{})
	err := newlyTx.Serialize(buf)
	if err != nil {
		leaderLogger.Error("serialize newly tx failed", "err", err)
		return nil
	}
	return &pb.NewlyTx{Data: buf.Bytes()}
}

func (ld *Leader) createEthInput(watchedTx *pb.WatchedTxInfo) *pb.NewlyTx {
	//input, err := ld.ethWatcher.EncodeMint(watchedTx.From, uint64(watchedTx.RechargeList[0].Amount),
	// 	watchedTx.RechargeList[0].Address, watchedTx.Txid+strconv.FormatInt(util.NowMs(), 10))
	addredss := ew.HexToAddress(watchedTx.RechargeList[0].Address)
	input, err := ld.ethWatcher.EncodeInput(ew.VOTE_METHOD_MINT, watchedTx.TokenTo, uint64(watchedTx.RechargeList[0].Amount),
		addredss, watchedTx.Txid)
	if err != nil {
		leaderLogger.Error("create eth input failed", "err", err, "sctxid", watchedTx.Txid)
		return nil
	}
	return &pb.NewlyTx{Data: input}
}

// 广播签名交易, 对于ETH，广播给其他节点即可；对于BTC/BCH，广播之后还需要收集返回的签名，按顺序merge之后去公链上发送交易
func (ld *Leader) broadcastSign(msg *pb.SignTxRequest, nodes []cluster.NodeInfo, quorumN int) {
	//对QuorumnN个节点可用才发送sign请求
	for availableCnt := ld.pm.GetTxConnAvailableCnt(nodes); availableCnt < quorumN; {
		leaderLogger.Debug("txConn is not available")
		time.Sleep(100 * time.Millisecond)
		availableCnt = ld.pm.GetTxConnAvailableCnt(nodes)
	}
	for _, node := range nodes {
		if node.IsNormal {
			go ld.pm.NotifySignTx(node.Id, msg)
		}
	}
}
