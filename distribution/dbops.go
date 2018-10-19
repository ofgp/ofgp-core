package distribution

import (
	"encoding/json"

	"github.com/ofgp/ofgp-core/dgwdb"
)

var (
	keyDistributionProposalPrefix = []byte("DPP")
)

// setDistributionProposal 保存分配提案
func setDistributionProposal(db *dgwdb.LDBDatabase, proposalID string, proposal *DistributionInfo) {
	key := append(keyDistributionProposalPrefix, []byte(proposalID)...)
	data, _ := json.Marshal(proposal)
	db.Put(key, data)
}

// getDistributionProposal 读取分配提案的数据
func getDistributionProposal(db *dgwdb.LDBDatabase, proposalID string) (*DistributionInfo, error) {
	key := append(keyDistributionProposalPrefix, []byte(proposalID)...)
	data, err := db.Get(key)
	if err != nil {
		return nil, err
	}
	rst := new(DistributionInfo)
	err = json.Unmarshal(data, rst)
	if err != nil {
		return nil, err
	}
	return rst, nil
}

// getAllDistributionProposal 读取所有的分配提案
func getAllDistributionProposal(db *dgwdb.LDBDatabase) []*DistributionInfo {
	var res []*DistributionInfo
	iter := db.NewIteratorWithPrefix(keyDistributionProposalPrefix)
	for iter.Next() {
		tmp := new(DistributionInfo)
		err := json.Unmarshal(iter.Value(), tmp)
		if err != nil {
			continue
		}
		res = append(res, tmp)
	}
	return res
}

// delDistributionProposal 删除指定的分配提案
func delDistributionProposal(db *dgwdb.LDBDatabase, proposalID string) {
	key := append(keyDistributionProposalPrefix, []byte(proposalID)...)
	db.Delete(key)
}
