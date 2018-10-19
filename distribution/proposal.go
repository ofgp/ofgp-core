package distribution

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"

	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/ofgp/ofgp-core/primitives"
	pb "github.com/ofgp/ofgp-core/proto"
)

const (
	proposalStatus = iota
	proposalNew
	proposalDealing
	proposalSuccess
	proposalFailed
)

// Output 一笔提案里面的一个输出
type Output struct {
	Address string `json:"address"`
	Amount  int64  `json:"amount"`
}

// Proposal 提案内容
type Proposal struct {
	Data       []Output `json:"data"`
	Chain      string   `json:"chain"`
	CreateTime string   `json:"create_time"`
	Status     int      `json:"status"`
}

// ProposalManager 提案管理，负责增删查
type ProposalManager struct {
	db           *dgwdb.LDBDatabase
	ts           *primitives.TxStore
	proposalInfo map[string]int
}

// NewProposalManager 新建一个ProposalManager实例并返回
func NewProposalManager(db *dgwdb.LDBDatabase, ts *primitives.TxStore) *ProposalManager {
	return &ProposalManager{
		db:           db,
		ts:           ts,
		proposalInfo: make(map[string]int),
	}
}

// AddProposal 新增一个提案
func (pm *ProposalManager) AddProposal(p *Proposal) (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	s := md5.Sum(data)
	proposalID := hex.EncodeToString(s[:])
	setDistributionProposal(pm.db, proposalID, data)

	watchedTx := genWatchedTx(p, proposalID)
	pm.ts.AddWatchedTx(watchedTx)
	return proposalID, nil
}

// DeleteProposal 删除一个提案
func (pm *ProposalManager) DeleteProposal(proposalID string) {
	delDistributionProposal(pm.db, proposalID)
}

// GetProposal 查找指定提案
func (pm *ProposalManager) GetProposal(proposalID string) *Proposal {
	data := getDistributionProposal(pm.db, proposalID)
	if data == nil {
		return nil
	}
	var res *Proposal
	err := json.Unmarshal(data, res)
	if err != nil {
		return nil
	}
	return res
}

// ListProposal 返回所有的提案
func (pm *ProposalManager) ListProposal() []*Proposal {
	var res []*Proposal
	data := getAllDistributionProposal(pm.db)
	for _, d := range data {
		var tmp *Proposal
		err := json.Unmarshal(d, tmp)
		if err != nil {
			continue
		}
		res = append(res, tmp)
	}
	return res
}

// ExecuteProposal 执行指定的提案
func (pm *ProposalManager) ExecuteProposal(proposalID string) {

}

func (pm *ProposalManager) SetFailed(proposalID string) {

}

func (pm *ProposalManager) SetSuccess(proposalID string) {

}

func (pm *ProposalManager) SetDealing(proposalID string) {

}

func genWatchedTx(p *Proposal, pID string) *pb.WatchedTxInfo {
	res := &pb.WatchedTxInfo{
		Txid: "DistributionTx" + pID,
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
