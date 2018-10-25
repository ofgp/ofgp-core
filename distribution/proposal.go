package distribution

import (
	"time"

	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/ofgp/ofgp-core/log"
	"github.com/ofgp/ofgp-core/primitives"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/spf13/viper"
)

const (
	proposalStatus = iota
	proposalNew
	proposalDealing
	proposalSuccess
	proposalFailed
)

var (
	logger = log.New(viper.GetString("loglevel"), "dist")
)

// Output 一笔提案里面的一个输出
type Output struct {
	Address string `json:"address"`
	Amount  int64  `json:"amount"`
}

// Proposal 提案内容
type Proposal struct {
	Data  []Output `json:"data"`
	Chain string   `json:"chain"`
	ID    string   `json:"id"`
}

type DistributionInfo struct {
	Proposal   *Proposal `json:"proposal"`
	CreateTime time.Time `json:"create_time"`
	Status     int       `json:"status"`
}

// ProposalManager 提案管理，负责增删查
type ProposalManager struct {
	db *dgwdb.LDBDatabase
	ts *primitives.TxStore
}

// NewProposalManager 新建一个ProposalManager实例并返回
func NewProposalManager(db *dgwdb.LDBDatabase, ts *primitives.TxStore) *ProposalManager {
	pm := &ProposalManager{
		db: db,
		ts: ts,
	}
	return pm
}

// LoadFromDB 把保存在磁盘里面的未完成的proposal读取到内存
func (pm *ProposalManager) LoadFromDB() {
	diList := getAllDistributionProposal(pm.db)
	for _, di := range diList {
		if di.Status != proposalSuccess {
			watchedTx := genWatchedTx(di.Proposal)
			pm.ts.AddWatchedTx(watchedTx)
			// 重启之后从磁盘读取，如果之前状态是处理中，修改为失败
			if di.Status == proposalDealing {
				pm.setStatus(di.Proposal.ID, proposalFailed)
			}
		}
	}
}

// AddProposal 新增一个提案
func (pm *ProposalManager) AddProposal(p *Proposal) bool {
	old, _ := getDistributionProposal(pm.db, p.ID)
	if old != nil {
		return false
	}
	di := &DistributionInfo{
		Proposal:   p,
		CreateTime: time.Now(),
		Status:     proposalNew,
	}
	setDistributionProposal(pm.db, di)

	watchedTx := genWatchedTx(p)
	pm.ts.AddWatchedTx(watchedTx)
	return true
}

// DeleteProposal 删除一个提案, 返回是否删除完成
func (pm *ProposalManager) DeleteProposal(proposalID string) bool {
	info, err := getDistributionProposal(pm.db, proposalID)
	if err != nil {
		return true
	}
	if info.Status == proposalFailed || info.Status == proposalNew {
		delDistributionProposal(pm.db, proposalID)
		return true
	}
	return false
}

// GetProposal 查找指定提案
func (pm *ProposalManager) GetProposal(proposalID string) *DistributionInfo {
	info, err := getDistributionProposal(pm.db, proposalID)
	if err != nil {
		return nil
	}
	return info
}

// ListProposal 返回所有的提案
func (pm *ProposalManager) ListProposal() []*DistributionInfo {
	return getAllDistributionProposal(pm.db)
}

// ExecuteProposal 执行指定的提案
func (pm *ProposalManager) ExecuteProposal(proposalID string) {
	currStatus := pm.getStatus(proposalID)
	if currStatus != proposalFailed && currStatus != proposalNew {
		return
	}
	pm.ts.WatchedTxToFresh("DistributionTx" + proposalID)
	pm.setStatus(proposalID, proposalDealing)
}

func (pm *ProposalManager) getStatus(proposalID string) int {
	info, err := getDistributionProposal(pm.db, proposalID)
	if err != nil {
		return proposalStatus
	}
	return info.Status
}

func (pm *ProposalManager) setStatus(proposalID string, status int) {
	logger.Debug("set proposal status", "proposal", proposalID, "status", status)
	info, err := getDistributionProposal(pm.db, proposalID)
	if err != nil {
		return
	}
	info.Status = status
	setDistributionProposal(pm.db, info)
}

// SetFailed 设置提案状态为执行失败
func (pm *ProposalManager) SetFailed(proposalID string) {
	pm.setStatus(proposalID, proposalFailed)
}

// SetSuccess 设置提案状态为执行成功
func (pm *ProposalManager) SetSuccess(proposalID string) {
	pm.setStatus(proposalID, proposalSuccess)
}

// SetDealing 设置提案状态为执行中
func (pm *ProposalManager) SetDealing(proposalID string) {
	pm.setStatus(proposalID, proposalDealing)
}

func genWatchedTx(p *Proposal) *pb.WatchedTxInfo {
	res := &pb.WatchedTxInfo{
		Txid: "DistributionTx" + p.ID,
		From: p.Chain,
		To:   p.Chain,
	}
	for _, o := range p.Data {
		res.RechargeList = append(res.RechargeList, &pb.AddressInfo{
			Address: o.Address,
			Amount:  o.Amount,
		})
	}
	return res
}
