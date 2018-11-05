package accuser

import (
	"context"
	"time"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/log"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/spf13/viper"
)

const (
	heartbeatInterval = 2 * time.Second
	accuseCooldown    = 10 * time.Second

	defaultHeatbeatIntervalTolerance = 60 * time.Second
	maxHeatbeatIntervalTolerance     = 3 * 60 * time.Second
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
	heatbeatSucChan  chan *pb.HeatbeatMsg // heatbeat 消息验证通过
	heatbeatFailChan chan struct{}        // heatbeat 验证失败
	heartbeatTimer   *time.Timer
	lastAccuseTime   time.Time
	lastAccuseTerm   int64

	termToAccuse int64

	hasHeatbeatInCurrentTerm bool //当前term是否收到heatbeat消息

	lastHeatbeatTime           time.Time
	heartbeatIntervalTolerance time.Duration //leader 心跳间隔
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
		heatbeatSucChan:  make(chan *pb.HeatbeatMsg),
		heatbeatFailChan: make(chan struct{}),
		heartbeatTimer:   time.NewTimer(heartbeatInterval),

		lastAccuseTime: time.Now().Add(-2 * accuseCooldown),
		lastAccuseTerm: -1,

		termToAccuse:               0,
		lastHeatbeatTime:           time.Now(),
		heartbeatIntervalTolerance: defaultHeatbeatIntervalTolerance,
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
// func (ac *Accuser) OnNewCommitted(newTop *pb.BlockPack) {
// 	ac.newCommittedChan <- newTop
// }

// OnHeatbeatSuc receive heatbeat from leader
func (ac *Accuser) OnHeatbeatSuc(msg *pb.HeatbeatMsg) {
	ac.heatbeatSucChan <- msg
}

// OnHeatbeatFail receive heatbeat from leader check fail
func (ac *Accuser) OnHeatbeatFail(msg struct{}) {
	ac.heatbeatFailChan <- msg
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
		case <-ac.heatbeatFailChan:
			ac.accuse(ac.termToAccuse, "heat beat fail", time.Now().Unix())
		case heatBeatMsg := <-ac.heatbeatSucChan:
			acLogger.Info("receive heatbeat msg, update lastHeatbeattime")
			ac.termToAccuse = heatBeatMsg.Term
			ac.lastHeatbeatTime = time.Now()
			ac.hasHeatbeatInCurrentTerm = true

		case newTerm := <-ac.newTermChan:
			acLogger.Debug("enter new term", "term", newTerm)
			ac.termToAccuse = newTerm
		case <-ac.heartbeatTimer.C:
			ac.heartbeatTimer.Reset(heartbeatInterval)
			now := time.Now()
			if now.After(ac.lastHeatbeatTime.Add(ac.heartbeatIntervalTolerance)) {
				if !ac.hasHeatbeatInCurrentTerm && ac.termToAccuse > ac.lastAccuseTerm {
					ac.heartbeatIntervalTolerance *= 2
					if ac.heartbeatIntervalTolerance > maxHeatbeatIntervalTolerance {
						ac.heartbeatIntervalTolerance = maxHeatbeatIntervalTolerance
					}
				}
				acLogger.Debug("heatbeat timeout accuse", "now", now, "last", ac.lastHeatbeatTime, "tole", ac.heartbeatIntervalTolerance)
				ac.accuse(ac.termToAccuse, "heatbeat timeout", time.Now().Unix())
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
