package accuser

import (
	"context"
	"time"

	pb "github.com/ofgp/ofgp-core/proto"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/log"

	"github.com/spf13/viper"
)

const (
	heartbeatInterval             = 2 * time.Second
	accuseCooldown                = 10 * time.Second
	defaultBlockIntervalTolerance = 60 * time.Second
	maxBlockIntervalTolerance     = 3 * 60 * time.Second
)

var (
	acLogger       = log.New(viper.GetString("loglevel"), "accuser")
	AccuseInterval int64 //accuse 间隔 单位s
)

// Accuser 发起accuse的结构体
type Accuser struct {
	localNode   cluster.NodeInfo
	signer      *crypto.SecureSigner
	peerManager *cluster.PeerManager

	bsTriggerChan    chan int64 //from blockstore, sure to accuse
	tsTriggerChan    chan int64
	newCommittedChan chan *pb.BlockPack
	newTermChan      chan int64
	heartbeatTimer   *time.Timer
	lastAccuseTime   time.Time
	lastAccuseTerm   int64

	termToAccuse              int64
	lastTermBlockTime         time.Time
	hasCommittedInCurrentTerm bool
	blockIntervalTolerance    time.Duration
}

// NewAccuser 新建一个Accuser对象并返回
func NewAccuser(nodeInfo cluster.NodeInfo, signer *crypto.SecureSigner,
	pm *cluster.PeerManager) *Accuser {
	ac := &Accuser{
		localNode:   nodeInfo,
		signer:      signer,
		peerManager: pm,

		bsTriggerChan:    make(chan int64),
		tsTriggerChan:    make(chan int64),
		newCommittedChan: make(chan *pb.BlockPack),
		newTermChan:      make(chan int64),
		heartbeatTimer:   time.NewTimer(heartbeatInterval),

		lastAccuseTime: time.Now().Add(-2 * accuseCooldown),
		lastAccuseTerm: -1,

		termToAccuse:              0,
		lastTermBlockTime:         time.Now(),
		hasCommittedInCurrentTerm: false,
		blockIntervalTolerance:    defaultBlockIntervalTolerance,
	}
	return ac
}

// TriggerByBlockStore blockstore发起accuse
func (ac *Accuser) TriggerByBlockStore(term int64) {
	ac.bsTriggerChan <- term
}

// TriggerByTxStore txstore发起accuse
func (ac *Accuser) TriggerByTxStore(term int64) {
	ac.tsTriggerChan <- term
}

// OnNewCommitted 新区块commit之后的回调处理
func (ac *Accuser) OnNewCommitted(newTop *pb.BlockPack) {
	ac.newCommittedChan <- newTop
}

// OnTermChange term自增之后的回调处理
func (ac *Accuser) OnTermChange(newTerm int64) {
	ac.newTermChan <- newTerm
}

// Run Accuser循环处理accuse并做区块间隔检测
func (ac *Accuser) Run(ctx context.Context) {
	for {
		select {
		case term := <-ac.bsTriggerChan:
			ac.accuse(term, "Invalid block", time.Now().Unix())
		case term := <-ac.tsTriggerChan:
			ac.accuse(term, "tx timeout", time.Now().Unix())

		case newTop := <-ac.newCommittedChan:
			acLogger.Info("new committed event, update lastTermBlockTime")
			ac.termToAccuse = newTop.Term()
			ac.lastTermBlockTime = time.Now()
			ac.hasCommittedInCurrentTerm = true
			ac.blockIntervalTolerance = defaultBlockIntervalTolerance

		case newTerm := <-ac.newTermChan:
			acLogger.Debug("enter new term", "term", newTerm)
			ac.termToAccuse = newTerm
			ac.lastTermBlockTime = time.Now() //wait the first commit from now
			ac.hasCommittedInCurrentTerm = false

		case <-ac.heartbeatTimer.C:
			ac.heartbeatTimer.Reset(heartbeatInterval)
			now := time.Now()
			if now.After(ac.lastTermBlockTime.Add(ac.blockIntervalTolerance)) {
				// 当tolerance时间内都没有新区块能共识，就accuse
				if !ac.hasCommittedInCurrentTerm && ac.termToAccuse > ac.lastAccuseTerm {
					// 防止网络很差的时候节点不停的提升term来选主节点
					ac.blockIntervalTolerance *= 2
					if ac.blockIntervalTolerance > maxBlockIntervalTolerance {
						ac.blockIntervalTolerance = maxBlockIntervalTolerance
					}
				}
				acLogger.Debug("block timeout accuse", "now", now, "last", ac.lastTermBlockTime, "tole", ac.blockIntervalTolerance)
				ac.accuse(ac.termToAccuse, "Block interval too long", time.Now().Unix())
				break
			}
		case <-ctx.Done():
			return
		}
	}
}

func (ac *Accuser) accuse(term int64, reason string, accuseTime int64) {
	acLogger.Debug("begin make a weak accuse", "reason", reason)
	if term < ac.lastAccuseTerm {
		return
	}
	if term == ac.lastAccuseTerm && time.Now().Before(ac.lastAccuseTime.Add(accuseCooldown)) {
		return
	}
	if AccuseInterval == 0 {
		AccuseInterval = 180
	}
	if accuseTime-ac.lastAccuseTime.Unix() < AccuseInterval {
		acLogger.Debug("accuse too quickly", "term", term, "reason", reason, "time", accuseTime)
		return
	}
	ac.lastAccuseTerm = term
	ac.lastAccuseTime = time.Now()

	wa, err := pb.MakeWeakAccuse(term, ac.localNode.Id, ac.signer)
	if err != nil {
		acLogger.Error("make weak accuse failed", "err", err)
		return
	}

	ac.peerManager.Broadcast(wa, false, false)
}
