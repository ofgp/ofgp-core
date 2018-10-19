package distribution

import (
	"github.com/ofgp/ofgp-core/dgwdb"
)

var (
	keyDistributionProposalPrefix = []byte("DPP")
)

// setDistributionProposal 保存分配提案
func setDistributionProposal(db *dgwdb.LDBDatabase, proposalID string, proposal []byte) {
	key := append(keyDistributionProposalPrefix, []byte(proposalID)...)
	db.Put(key, proposal)
}

// getDistributionProposal 读取分配提案的数据
func getDistributionProposal(db *dgwdb.LDBDatabase, proposalID string) []byte {
	key := append(keyDistributionProposalPrefix, []byte(proposalID)...)
	data, _ := db.Get(key)
	return data
}

// getAllDistributionProposal 读取所有的分配提案
func getAllDistributionProposal(db *dgwdb.LDBDatabase) [][]byte {
	var res [][]byte
	iter := db.NewIteratorWithPrefix(keyDistributionProposalPrefix)
	for iter.Next() {
		res = append(res, iter.Value())
	}
	return res
}

// delDistributionProposal 删除指定的分配提案
func delDistributionProposal(db *dgwdb.LDBDatabase, proposalID string) {
	key := append(keyDistributionProposalPrefix, []byte(proposalID)...)
	db.Delete(key)
}
