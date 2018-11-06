package proto

import (
	"encoding/json"
	"eosc/eoswatcher"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/ofgp/bitcoinWatcher/coinmanager"
	btcwatcher "github.com/ofgp/bitcoinWatcher/mortgagewatcher"
	"github.com/ofgp/ethwatcher"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/util"
	"github.com/ofgp/ofgp-core/util/assert"
	"github.com/ofgp/ofgp-core/util/sort"
)

// EqualTo 会更新两个ID，再进行比较。比较函数里面进行更新不是很合理的说
func (tx *Transaction) EqualTo(other *Transaction) bool {
	tx.UpdateId()
	other.UpdateId()
	return tx.Id.EqualTo(other.Id)
}

// UpdateId 更新Tx的ID
func (tx *Transaction) UpdateId() {
	hasher := crypto.NewHasher256()
	feedTxFields(hasher, "Tx", tx)
	tx.Id = hasher.Sum(nil)
}

// EqualTo 比较两个WatchedTxInfo内容是否相等
func (tx *WatchedTxInfo) EqualTo(other *WatchedTxInfo) bool {
	if !(tx.Txid == other.Txid && tx.Amount == other.Amount && tx.From == other.From &&
		tx.To == other.To) {
		return false
	}
	if len(tx.RechargeList) != len(other.RechargeList) {
		return false
	}
	for idx, recharge := range tx.RechargeList {
		if !(recharge.Address == other.RechargeList[idx].Address &&
			recharge.Amount == other.RechargeList[idx].Amount) {
			return false
		}
	}
	return true
}

//Clone 深拷贝一个WatchedTxInfo对象
func (tx *WatchedTxInfo) Clone() *WatchedTxInfo {
	if tx == nil {
		return nil
	}
	return proto.Clone(tx).(*WatchedTxInfo)

}

//UnmarshalBchBlockHeader 反序列化一个pb对象
func UnmarshalBchBlockHeader(bytes []byte) *BchBlockHeader {
	rst := new(BchBlockHeader)
	err := proto.Unmarshal(bytes, rst)
	assert.ErrorIsNil(err)
	return rst
}

func (header *BchBlockHeader) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedBchBlockHeaderFields(hasher, "BchBlockHeader", header)
	return hasher.Sum(nil)
}

func (header *BchBlockHeader) IsPrevOf(other *BchBlockHeader) bool {
	return header.Height+1 == other.Height && header.BlockId.EqualTo(other.PrevId)
}

//------Block

func (reconfig *Reconfig) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedReconfigFields(hasher, "Reconfig", reconfig)
	return hasher.Sum(nil)
}

func CreateTxsBlock(timestampMs int64, prevBlockId *crypto.Digest256, txs []*Transaction) *Block {
	block := &Block{
		TimestampMs: timestampMs,
		PrevBlockId: prevBlockId,
		Type:        Block_TXS,
		Txs:         txs,
	}
	block.UpdateBlockId()
	return block
}

func CreateJoinReconfigBlock(timestampMs int64, prevBlockId *crypto.Digest256, t Reconfig_Type, host string, nodeId int32, pubkey string, vote *Vote) *Block {
	block := &Block{
		TimestampMs: timestampMs,
		PrevBlockId: prevBlockId,
		Type:        Block_RECONFIG,
		Reconfig: &Reconfig{
			Type:   t,
			Host:   host,
			NodeId: nodeId,
			Pubkey: pubkey,
			Vote:   vote,
		},
	}
	block.UpdateBlockId()
	return block
}

func CreateLeaveReconfigBlock(timestampMs int64, prevBlockId *crypto.Digest256, t Reconfig_Type, nodeId int32) *Block {
	block := &Block{
		TimestampMs: timestampMs,
		PrevBlockId: prevBlockId,
		Type:        Block_RECONFIG,
		Reconfig: &Reconfig{
			Type:   t,
			NodeId: nodeId,
		},
	}
	block.UpdateBlockId()
	return block
}

func CreateBchBlock(timestampMs int64, prevBlockId *crypto.Digest256,
	txs []*Transaction, header *BchBlockHeader) *Block {
	block := &Block{
		TimestampMs:    timestampMs,
		PrevBlockId:    prevBlockId,
		Type:           Block_BCH,
		Txs:            txs,
		BchBlockHeader: header,
	}
	block.UpdateBlockId()
	return block
}

func (b *Block) UpdateBlockId() {
	hasher := crypto.NewHasher256()
	feedBlockFields(hasher, "Block", b)
	b.Id = hasher.Sum(nil)
}

func (b *Block) IsTxsBlock() bool {
	return b.Type == Block_TXS
}

func (b *Block) IsGenesis() bool {
	return b.Type == Block_GENESIS
}

func (b *Block) IsBchBlock() bool {
	return b.Type == Block_BCH
}

func (b *Block) IsReconfig() bool {
	return b.Type == Block_RECONFIG
}

//----------------BlockInfo

func NewBlockInfoFull(term, height int64, block *Block) *BlockInfo {
	assert.True(term >= 0)
	assert.True(height >= 0)
	return &BlockInfo{
		Term:    term,
		Height:  height,
		BlockId: block.Id,
		Block:   block,
	}
}

func NewBlockInfoLite(term, height int64, blockId *crypto.Digest256) *BlockInfo {
	assert.True(term >= 0)
	assert.True(height >= 0)
	assert.True(blockId != nil)

	return &BlockInfo{
		Term:    term,
		Height:  height,
		BlockId: blockId,
	}
}

//-----------------BlockPack

func NewBlockPack(init *InitMsg) *BlockPack {
	assert.True(init != nil)
	return &BlockPack{
		Init:     init,
		Prepares: make(map[int32]*PrepareMsg),
		Commits:  make(map[int32]*CommitMsg),
	}
}

func (bp *BlockPack) makeMap() *BlockPack {
	if bp.Prepares == nil {
		bp.Prepares = make(map[int32]*PrepareMsg)
	}
	if bp.Commits == nil {
		bp.Commits = make(map[int32]*CommitMsg)
	}
	return bp
}

func UnmarshalBlockPack(bytes []byte) *BlockPack {
	rst := new(BlockPack)
	err := proto.Unmarshal(bytes, rst)
	assert.ErrorIsNil(err)
	return rst.makeMap()
}

func (bp *BlockPack) Clone() *BlockPack {
	if bp == nil {
		return nil
	}
	return proto.Clone(bp).(*BlockPack).makeMap()
}

func (bp *BlockPack) ShallowCopy() *BlockPack {
	if bp == nil {
		return nil
	}
	return &BlockPack{
		Init:     bp.Init,
		Prepares: bp.Prepares,
		Commits:  bp.Commits,
	}
}

func (bp *BlockPack) Term() int64 {
	return bp.Init.Term
}

func (bp *BlockPack) Height() int64 {
	return bp.Init.Height
}

func (bp *BlockPack) Block() *Block {
	return bp.Init.Block
}

func (bp *BlockPack) BlockId() *crypto.Digest256 {
	return bp.Block().Id
}

func (bp *BlockPack) TimestampMs() int64 {
	return bp.Block().TimestampMs
}

func (bp *BlockPack) PrevBlockId() *crypto.Digest256 {
	return bp.Block().PrevBlockId
}

func (bp *BlockPack) BlockInfo() *BlockInfo {
	return NewBlockInfoFull(bp.Term(), bp.Height(), bp.Block())
}

func (bp *BlockPack) IsTxsBlock() bool {
	return bp.Block().IsTxsBlock()
}

func (bp *BlockPack) IsBchBlock() bool {
	return bp.Block().IsBchBlock()
}

func (bp *BlockPack) IsReconfigBlock() bool {
	return bp.Block().IsReconfig()
}

func (bp *BlockPack) HasLessTermHeightThan(other *BlockPack) bool {
	return bp.Term() < other.Term() ||
		(bp.Term() == other.Term() && bp.Height() < other.Height())
}

func (bp *BlockPack) BlockIdEqualTo(other *BlockPack) bool {
	return bp.BlockId().EqualTo(other.BlockId())
}

func (bp *BlockPack) ToVotie() *Votie {
	return &Votie{
		Term:     bp.Term(),
		Height:   bp.Height(),
		Block:    bp.Block(),
		Prepares: bp.Prepares,
		Commits:  bp.Commits,
	}
}

func (bp *BlockPack) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedBlockPackFields(hasher, "BlockPack", bp)
	return hasher.Sum(nil)
}

func (bp *BlockPack) DebugString() string {
	desc := ""
	desc += fmt.Sprintf("[Term %d, Height %d] ", bp.Term(), bp.Height())
	desc += fmt.Sprintf("[Init from %d] ", bp.Init.NodeId)

	var prepares []int32
	for _, prepare := range bp.Prepares {
		prepares = append(prepares, prepare.NodeId)
	}
	sort.Int32s(prepares, false)
	desc += fmt.Sprintf("[Prepares %v] ", prepares)

	var commits []int32
	for _, commit := range bp.Commits {
		commits = append(commits, commit.NodeId)
	}
	sort.Int32s(commits, false)
	desc += fmt.Sprintf("[Commits %v]", commits)

	return desc
}

//----------------------Votie

func UnmarshalVotie(bytes []byte) *Votie {
	rst := new(Votie)
	err := proto.Unmarshal(bytes, rst)
	assert.ErrorIsNil(err)
	return rst
}

func (v *Votie) HasLessTermHeightThan(other *Votie) bool {
	return v.Term < other.Term ||
		(v.Term == other.Term && v.Height < other.Height)
}

func (v *Votie) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedVotieFields(hasher, "Votie", v)
	return hasher.Sum(nil)
}

//-----------------Vote

func GetMaxVotie(votes map[int32]*Vote) *Votie {
	var rst *Votie
	for _, vote := range votes {
		if rst == nil || rst.HasLessTermHeightThan(vote.Votie) {
			rst = vote.Votie
		}
	}
	return rst
}

type VoteMap map[int32]*Vote //map: nodeId -> *vote

func (vm VoteMap) GetMaxVotie() *Votie {
	return GetMaxVotie(vm)
}

func (vote *Vote) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedVoteFields(hasher, "Vote", vote)
	return hasher.Sum(nil)
}

func (vote *Vote) DebugString() string {
	desc := ""
	desc += fmt.Sprintf("[Vote term: %d, Node Id: %d]", vote.Term, vote.NodeId)
	votie := vote.Votie
	desc += fmt.Sprintf("[Votie term: %d, Height: %d]", votie.GetTerm(), votie.Height)
	var prepares []int32
	for _, prepare := range votie.Prepares {
		prepares = append(prepares, prepare.NodeId)
	}
	desc += fmt.Sprintf("[Prepares: %v]", prepares)
	var commits []int32
	for _, commit := range votie.Commits {
		commits = append(commits, commit.NodeId)
	}
	desc += fmt.Sprintf("[Commits: %v]", commits)
	desc += fmt.Sprintf("[Siglen: %d]", len(vote.Sig))

	return desc
}

func MakeVote(term int64, votie *Votie, nodeId int32,
	signer *crypto.SecureSigner) (*Vote, error) {
	vote := &Vote{
		Term:   term,
		Votie:  votie,
		NodeId: nodeId,
	}
	sig, err := signer.Sign(vote.Id().Data)
	if err != nil {
		return nil, err
	}
	vote.Sig = sig
	return vote, nil
}

//----------------------InitMsg

func (init *InitMsg) BlockId() *crypto.Digest256 {
	return init.Block.Id
}

func (init *InitMsg) PrevBlockId() *crypto.Digest256 {
	return init.Block.PrevBlockId
}

func (init *InitMsg) BlockInfo() *BlockInfo {
	return NewBlockInfoFull(init.Term, init.Height, init.Block)
}

func (init *InitMsg) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedInitMsgFields(hasher, "InitMsg", init)
	return hasher.Sum(nil)
}

//--------------------PrepareMsg

func MakePrepareMsg(blockInfo *BlockInfo, nodeId int32, signer *crypto.SecureSigner) (*PrepareMsg, error) {
	msg := &PrepareMsg{
		Term:    blockInfo.Term,
		Height:  blockInfo.Height,
		BlockId: blockInfo.BlockId,
		NodeId:  nodeId,
	}
	sig, err := signer.Sign(msg.Id().Data)
	if err != nil {
		return nil, err
	}
	msg.Sig = sig
	return msg, nil
}

func (msg *PrepareMsg) DebugString() string {
	return fmt.Sprintf("[Term: %d, Height: %d, BlockIdLen: %d, NodeId: %d]", msg.Term, msg.Height,
		len(msg.BlockId.Data), msg.NodeId)
}

func (msg *PrepareMsg) BlockInfoLite() *BlockInfo {
	return NewBlockInfoLite(msg.Term, msg.Height, msg.BlockId)
}

func (msg *PrepareMsg) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedPrepareMsgFields(hasher, "PrepareMsg", msg)
	return hasher.Sum(nil)
}

//-----------------CommitMsg

func MakeCommitMsg(blockInfo *BlockInfo, nodeId int32, signer *crypto.SecureSigner) (*CommitMsg, error) {
	msg := &CommitMsg{
		Term:    blockInfo.Term,
		Height:  blockInfo.Height,
		BlockId: blockInfo.BlockId,
		NodeId:  nodeId,
	}
	sig, err := signer.Sign(msg.Id().Data)
	if err != nil {
		return nil, err
	}
	msg.Sig = sig
	return msg, nil
}

func (msg *CommitMsg) BlockInfoLite() *BlockInfo {
	return NewBlockInfoLite(msg.Term, msg.Height, msg.BlockId)
}

func (msg *CommitMsg) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedCommitMsgFields(hasher, "CommitMsg", msg)
	return hasher.Sum(nil)
}

//-------------------WeakAccuse

func MakeWeakAccuse(term int64, nodeId int32, signer *crypto.SecureSigner) (*WeakAccuse, error) {
	wa := &WeakAccuse{
		Term:   term,
		NodeId: nodeId,
		Time:   time.Now().Unix(),
	}
	sig, err := signer.Sign(wa.Id().Data)
	if err != nil {
		return nil, err
	}
	wa.Sig = sig
	return wa, nil
}

func EmptyWeakAccuses() *WeakAccuses {
	rst := new(WeakAccuses)
	rst.Accuses = make(map[int32]*WeakAccuse)
	return rst
}

func UnmarshalWeakAccuses(bytes []byte) *WeakAccuses {
	rst := new(WeakAccuses)
	err := proto.Unmarshal(bytes, rst)
	assert.ErrorIsNil(err)
	if rst.Accuses == nil {
		rst.Accuses = make(map[int32]*WeakAccuse)
	}
	return rst
}

func (wa *WeakAccuses) Size() int {
	return len(wa.Accuses)
}

func (wa *WeakAccuses) Get(nodeId int32) *WeakAccuse {
	return wa.Accuses[nodeId]
}

func (wa *WeakAccuses) Set(nodeId int32, weakAccuse *WeakAccuse) {
	wa.Accuses[nodeId] = weakAccuse
}

func (wa *WeakAccuse) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedWeakAccuseFields(hasher, "WeakAccuse", wa)
	return hasher.Sum(nil)
}

//-------------------StrongAccuse

func NewStrongAccuse(weakAccuses map[int32]*WeakAccuse) *StrongAccuse {
	rst := &StrongAccuse{
		WeakAccuses: make(map[int32]*WeakAccuse),
	}
	for nodeId, weakAccuse := range weakAccuses {
		rst.WeakAccuses[nodeId] = weakAccuse
	}
	return rst
}

func UnmarshalStrongAccuse(bytes []byte) *StrongAccuse {
	rst := new(StrongAccuse)
	err := proto.Unmarshal(bytes, rst)
	assert.ErrorIsNil(err)
	if rst.WeakAccuses == nil {
		rst.WeakAccuses = make(map[int32]*WeakAccuse)
	}
	return rst
}

func (sa *StrongAccuse) Term() int64 {
	for _, weakAccuse := range sa.WeakAccuses {
		return weakAccuse.Term
	}
	panic("empty strong accuse")
}

func (sa *StrongAccuse) DebugString() string {
	var voters []int32
	for voter := range sa.WeakAccuses {
		voters = append(voters, voter)
	}
	sort.Int32s(voters, false)
	return fmt.Sprintf("Term %d, Voters %v", sa.Term(), voters)
}

func feedBchBlockHeaderFields(hasher *crypto.Hasher256, fieldName string, header *BchBlockHeader) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedInt32Field(hs, "Height", header.Height)
		util.FeedDigestField(hs, "BlockId", header.BlockId)
		util.FeedDigestField(hs, "PrevId", header.PrevId)
	})
}

func feedBlockFields(hasher *crypto.Hasher256, fieldName string, block *Block) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedTimestampField(hs, "TimestampMs", block.TimestampMs)
		if !block.IsGenesis() {
			util.FeedDigestField(hs, "PreviousBlockId", block.PrevBlockId)
		}
		util.FeedTextField(hs, "Type", block.Type.String())

		switch block.Type {
		case Block_GENESIS:
		case Block_TXS:
			util.FeedField(hs, "TxIds", func(h *crypto.Hasher256) {
				for _, tx := range block.Txs {
					util.FeedDigestField(h, "TxId", tx.Id)
				}
			})
		case Block_BCH:
			util.FeedField(hs, "TxIds", func(h *crypto.Hasher256) {
				for _, tx := range block.Txs {
					util.FeedDigestField(h, "TxId", tx.Id)
				}
			})
			util.FeedDigestField(hs, "BchBlockHeader", block.BchBlockHeader.Id())
		case Block_RECONFIG:
			util.FeedDigestField(hs, "ReconfigId", block.Reconfig.Id())
		default:
			panic("Invalid block type.")
		}
	})
}

func feedBlockPackFields(hasher *crypto.Hasher256, fieldName string, bp *BlockPack) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedDigestField(hs, "InitMsgId", bp.Init.Id())
		util.FeedField(hs, "Prepares", func(hs *crypto.Hasher256) {
			mapKeys := make([]int32, 0, len(bp.Prepares))
			for id := range bp.Prepares {
				mapKeys = append(mapKeys, id)
			}
			sort.Int32s(mapKeys, false)
			for _, k := range mapKeys {
				util.FeedDigestField(hs, "PrepareMsgId", bp.Prepares[k].Id())
			}
		})
		util.FeedField(hs, "Commits", func(hs *crypto.Hasher256) {
			mapKeys := make([]int32, 0, len(bp.Commits))
			for id := range bp.Commits {
				mapKeys = append(mapKeys, id)
			}
			sort.Int32s(mapKeys, false)
			for _, k := range mapKeys {
				util.FeedDigestField(hs, "CommitMsgId", bp.Commits[k].Id())
			}
		})
	})
}

func feedVotieFields(hasher *crypto.Hasher256, fieldName string, votie *Votie) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedInt64Field(hs, "Term", votie.Term)
		util.FeedInt64Field(hs, "Height", votie.Height)
		util.FeedDigestField(hs, "BlockId", votie.Block.Id)
		util.FeedField(hs, "Prepares", func(hs *crypto.Hasher256) {
			mapKeys := make([]int32, 0, len(votie.Prepares))
			for id := range votie.Prepares {
				mapKeys = append(mapKeys, id)
			}
			sort.Int32s(mapKeys, false)
			for _, k := range mapKeys {
				util.FeedDigestField(hs, "PrepareMsgId", votie.Prepares[k].Id())
			}
		})
		util.FeedField(hs, "Commits", func(hs *crypto.Hasher256) {
			mapKeys := make([]int32, 0, len(votie.Commits))
			for id := range votie.Commits {
				mapKeys = append(mapKeys, id)
			}
			sort.Int32s(mapKeys, false)
			for _, k := range mapKeys {
				util.FeedDigestField(hs, "CommitMsgId", votie.Commits[k].Id())
			}
		})
	})
}

func feedVoteFields(hasher *crypto.Hasher256, fieldName string, vote *Vote) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedInt64Field(hs, "Term", vote.Term)
		util.FeedDigestField(hs, "VotieId", vote.Votie.Id())
		util.FeedInt32Field(hs, "NodeId", vote.NodeId)
	})
}

func feedInitMsgFields(hasher *crypto.Hasher256, fieldName string, msg *InitMsg) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedInt64Field(hs, "Term", msg.Term)
		util.FeedInt64Field(hs, "Height", msg.Height)
		util.FeedDigestField(hs, "BlockId", msg.BlockId())
		util.FeedInt32Field(hs, "NodeId", msg.NodeId)

		if len(msg.Votes) > 0 {
			util.FeedField(hs, "Votes", func(hs *crypto.Hasher256) {
				mapKeys := make([]int32, 0, len(msg.Votes))
				for id := range msg.Votes {
					mapKeys = append(mapKeys, id)
				}
				sort.Int32s(mapKeys, false)
				for _, k := range mapKeys {
					util.FeedDigestField(hs, "VoteId", msg.Votes[k].Id())
				}
			})
		}
	})
}

func feedPrepareMsgFields(hasher *crypto.Hasher256, fieldName string, msg *PrepareMsg) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedInt64Field(hs, "Term", msg.Term)
		util.FeedInt64Field(hs, "Height", msg.Height)
		util.FeedDigestField(hs, "BlockId", msg.BlockId)
		util.FeedInt32Field(hs, "NodeId", msg.NodeId)
	})
}

func feedCommitMsgFields(hasher *crypto.Hasher256, fieldName string, msg *CommitMsg) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedInt64Field(hs, "Term", msg.Term)
		util.FeedInt64Field(hs, "Height", msg.Height)
		util.FeedDigestField(hs, "BlockId", msg.BlockId)
		util.FeedInt32Field(hs, "NodeId", msg.NodeId)
	})
}

func feedWeakAccuseFields(hasher *crypto.Hasher256, fieldName string, msg *WeakAccuse) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedInt64Field(hs, "Term", msg.Term)
		util.FeedInt32Field(hs, "NodeId", msg.NodeId)
	})
}

func feedTxFields(hasher *crypto.Hasher256, fieldName string, msg *Transaction) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedField(hs, "WatchedTx", func(hs *crypto.Hasher256) {
			tx := msg.WatchedTx
			util.FeedTextField(hs, "Txid", tx.Txid)
			util.FeedInt64Field(hs, "Amount", tx.Amount)
			util.FeedTextField(hs, "From", tx.From)
			util.FeedTextField(hs, "To", tx.To)
			if len(tx.RechargeList) > 0 {
				util.FeedField(hs, "Recharge", func(hs *crypto.Hasher256) {
					for _, recharge := range tx.RechargeList {
						util.FeedTextField(hs, "Address", recharge.Address)
						util.FeedInt64Field(hs, "Amount", recharge.Amount)
					}
				})
			}
		})
		util.FeedTextField(hs, "NewlyTxId", msg.NewlyTxId)
	})
}

func feedSignMsgFields(hasher *crypto.Hasher256, fieldName string, msg *SignTxRequest) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedInt64Field(hs, "Term", msg.Term)
		util.FeedInt32Field(hs, "NodeId", msg.NodeId)
		util.FeedTextField(hs, "MultisigAddress", msg.MultisigAddress)
		util.FeedField(hs, "WatchedTx", func(hs *crypto.Hasher256) {
			tx := msg.WatchedTx
			util.FeedTextField(hs, "Txid", tx.Txid)
			util.FeedInt64Field(hs, "Amount", tx.Amount)
			util.FeedTextField(hs, "From", tx.From)
			util.FeedTextField(hs, "To", tx.To)
			if len(tx.RechargeList) > 0 {
				util.FeedField(hs, "Recharge", func(hs *crypto.Hasher256) {
					for _, recharge := range tx.RechargeList {
						util.FeedTextField(hs, "Address", recharge.Address)
						util.FeedInt64Field(hs, "Amount", recharge.Amount)
					}
				})
			}
		})
		util.FeedBinField(hs, "NewlyTx", msg.NewlyTx.Data)
	})
}

func feedSignedResultFields(hasher *crypto.Hasher256, fieldName string, msg *SignedResult) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedInt32Field(hs, "Code", int32(msg.Code))
		util.FeedInt32Field(hs, "NodeId", msg.NodeId)
		util.FeedTextField(hs, "TxId", msg.TxId)
		util.FeedTextField(hs, "To", msg.To)
		util.FeedInt64Field(hs, "Term", msg.Term)
		if len(msg.Data) > 0 {
			util.FeedField(hs, "SignedData", func(hs *crypto.Hasher256) {
				for _, data := range msg.Data {
					util.FeedBinField(hs, "Data", data)
				}
			})
		}
	})
}

func feedChainTxIdMsgFields(hasher *crypto.Hasher256, fieldName string, msg *ChainTxIdMsg) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedInt32Field(hs, "NodeId", msg.NodeId)
		util.FeedTextField(hs, "TxId", msg.TxId)
		util.FeedTextField(hs, "SignMsgId", msg.SignMsgId)
	})
}

func feedReconfigFields(hasher *crypto.Hasher256, fieldName string, msg *Reconfig) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedInt32Field(hs, "Type", int32(msg.Type))
		util.FeedTextField(hs, "Host", msg.Host)
		util.FeedInt32Field(hs, "NodeId", msg.NodeId)
	})
}

func feedJoinRequestFields(hasher *crypto.Hasher256, fieldName string, msg *JoinRequest) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedTextField(hs, "Host", msg.Host)
		util.FeedTextField(hs, "Pubkey", msg.Pubkey)
		util.FeedDigestField(hs, "Vote", msg.Vote.Id())
	})
}

func feedLeaveRequestFields(hasher *crypto.Hasher256, fieldName string, msg *LeaveRequest) {
	util.FeedField(hasher, fieldName, func(hs *crypto.Hasher256) {
		util.FeedInt32Field(hs, "NodeId", msg.NodeId)
		util.FeedTextField(hs, "Msg", msg.Msg)
	})
}

// UnmarshalTxLookupEntry 反序列化TxLookupEntry
func UnmarshalTxLookupEntry(bytes []byte) *TxLookupEntry {
	rst := new(TxLookupEntry)
	err := proto.Unmarshal(bytes, rst)
	assert.ErrorIsNil(err)
	return rst
}

func (msg *SignedResult) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedSignedResultFields(hasher, "SignedResult", msg)
	return hasher.Sum(nil)
}

func MakeSignedResult(code CodeType, nodeId int32, txId string, data [][]byte,
	to string, term int64, signer *crypto.SecureSigner) (*SignedResult, error) {
	msg := &SignedResult{
		Code:   code,
		NodeId: nodeId,
		TxId:   txId,
		To:     to,
		Data:   data,
		Term:   term,
	}
	sig, err := signer.Sign(msg.Id().Data)
	if err != nil {
		return nil, err
	}
	msg.Sig = sig
	return msg, nil
}

func MakeSignedStatistic(msgId string, nodeId int32, code CodeType) *SignedStatistic {
	return &SignedStatistic{
		SignedMsgId: msgId,
		Stat:        map[int32]CodeType{nodeId: code},
		Status:      TxStatus_WAITING,
	}
}

func UnmarshalSignedStatistic(bytes []byte) *SignedStatistic {
	rst := new(SignedStatistic)
	err := proto.Unmarshal(bytes, rst)
	assert.ErrorIsNil(err)
	return rst
}

// BtcToPbTx BCH链监听的交易结构转成pb结构
// 会统一在这边做金额校验，如果输出金额比输入金额大，则按地址顺序修正金额，最后为0的输出地址被忽略
// 做金额修正主要是为了防止金额作弊的情况
func BtcToPbTx(tx *btcwatcher.SubTransaction) *WatchedTxInfo {
	if tx.Amount <= 0 {
		return nil
	}

	if len(tx.RechargeList) == 0 {
		return nil
	}

	watchedTx := &WatchedTxInfo{
		Txid:      tx.ScTxid,
		Amount:    tx.Amount,
		From:      tx.From,
		To:        tx.To,
		TokenFrom: tx.TokenFrom,
		TokenTo:   tx.TokenTo,
	}
	leftAmount := tx.Amount
	for _, addressInfo := range tx.RechargeList {
		if addressInfo.Amount <= 0 {
			return nil
		}
		amount := addressInfo.Amount
		if addressInfo.Amount > leftAmount {
			amount = leftAmount
		}
		watchedTx.RechargeList = append(watchedTx.RechargeList, &AddressInfo{
			Amount:  amount,
			Address: addressInfo.Address,
		})
		leftAmount -= addressInfo.Amount
		if leftAmount <= 0 {
			break
		}
	}
	return watchedTx
}

// EthToPbTx ETH链监听到的交易转pb结构。
func EthToPbTx(tx *ethwatcher.ExtraBurnData) *WatchedTxInfo {
	if tx.Amount <= 0 {
		log.Printf("tx amount is not right tx:%s", tx.ScTxid)
		return nil
	}

	if len(tx.RechargeList) == 0 {
		log.Printf("tx recharge list is empty tx:%s", tx.ScTxid)
		return nil
	}

	watchedTx := &WatchedTxInfo{
		Txid:      tx.ScTxid,
		Amount:    int64(tx.Amount),
		From:      tx.From,
		To:        tx.To,
		TokenFrom: tx.TokenFrom,
		TokenTo:   tx.TokenTo,
		Fee:       0,
	}
	leftAmount := tx.Amount
	for _, addressInfo := range tx.RechargeList {
		if addressInfo.Amount <= 0 {
			log.Printf("addrres:%s amount is <=0 tx:%s", addressInfo.Address, tx.ScTxid)
			return nil
		}
		amount := addressInfo.Amount
		if addressInfo.Amount > leftAmount {
			amount = leftAmount
		}

		_, err := coinmanager.DecodeAddress(addressInfo.Address, tx.To)
		if err != nil {
			log.Printf("address is illegal, tx: %s", tx.ScTxid)
			return nil
		}
		watchedTx.RechargeList = append(watchedTx.RechargeList, &AddressInfo{
			Amount:  int64(amount),
			Address: addressInfo.Address,
		})
		leftAmount -= addressInfo.Amount
		if leftAmount <= 0 {
			break
		}
	}
	return watchedTx
}

type eosMemo struct {
	Address string `json:"address"`
	Chain   string `json:"chain"`
}

// XINToPbTx XIN链监听到的交易转pb结构
func XINToPbTx(tx *eoswatcher.EOSPushEvent) *WatchedTxInfo {
	if tx.GetAmount() <= 0 {
		log.Printf("xin to pbtx amount <=0 amount:%d\n", tx.GetAmount())
		return nil
	}
	memo := &eosMemo{}
	err := json.Unmarshal(tx.GetData(), memo)
	if err != nil {
		log.Printf("unmarshal memo err:%v,data:%s\n", err, tx.GetData())
		return nil
	}
	if memo.Chain != "btc" && memo.Chain != "bch" && memo.Chain != "eos" {
		log.Printf("xin chain type err chain:%s\n", memo.Chain)
		return nil
	}
	if memo.Chain == "btc" || memo.Chain == "bch" {
		_, err = coinmanager.DecodeAddress(memo.Address, memo.Chain)
		if err != nil {
			log.Printf("xin decode adrr err:%v,addr:%s,chain:%s\n", err, memo.Address, memo.Chain)
			return nil
		}
	}
	if memo.Chain == "eos" {
		if !checkEosAddr(memo.Address) {
			log.Printf("eosEvent addr wrong addr:%s", memo.Address)
			return nil
		}
	}

	watchedTx := &WatchedTxInfo{
		Txid:      tx.GetTxID(),
		Amount:    int64(tx.GetAmount()),
		From:      "xin",
		To:        memo.Chain,
		TokenFrom: 1,
		TokenTo:   0,
		Fee:       0,
	}
	watchedTx.RechargeList = append(watchedTx.RechargeList, &AddressInfo{
		Amount:  int64(tx.GetAmount()),
		Address: memo.Address,
	})
	return watchedTx
}

func checkEosAddr(addr string) bool {
	reg := regexp.MustCompile(`^[a-z1-5.]+$`)
	if reg.MatchString(addr) {
		return true
	}
	return false
}

// EOSToPbTx eosEvent->watchedInfo
func EOSToPbTx(event *eoswatcher.EOSPushEvent) *WatchedTxInfo {
	if event.GetAmount() <= 0 {
		log.Printf("eos to pbtx amount<=0 amount:%d\n", event.GetAmount())
		return nil
	}
	memo := &eosMemo{}
	err := json.Unmarshal(event.GetData(), memo)
	if err != nil {
		log.Printf("unmarshal memo err:%v,data:%s\n", err, event.GetData())
		return nil
	}
	if !checkEosAddr(memo.Address) {
		log.Printf("eosEvent addr wrong addr:%s", memo.Address)
		return nil
	}
	watchedTx := &WatchedTxInfo{
		Txid:      event.GetTxID(),
		Amount:    int64(event.GetAmount()),
		From:      "eos",
		To:        memo.Chain,
		TokenFrom: 1,
		TokenTo:   0,
		Fee:       0,
	}
	return watchedTx
}

// IsTransferTx 判断WatchedTxInfo是否是一个多签地址的资产转移的交易
func (tx *WatchedTxInfo) IsTransferTx() bool {
	return tx.From == tx.To && strings.HasPrefix(tx.Txid, "TransferTx")
}

// IsDistributionTx 判断WatchedTxInfo是否是一个分配多签资产的交易
func (tx *WatchedTxInfo) IsDistributionTx() bool {
	return tx.From == tx.To && strings.HasPrefix(tx.Txid, "DistributionTx")
}

// MakeSignTxMsg 创建一个SignTxMsg并返回
func MakeSignTxMsg(term int64, nodeId int32, watchedTx *WatchedTxInfo, newlyTx *NewlyTx,
	multisigAddress string, signer *crypto.SecureSigner) (*SignTxRequest, error) {
	msg := &SignTxRequest{
		Term:            term,
		NodeId:          nodeId,
		MultisigAddress: multisigAddress,
		WatchedTx:       watchedTx,
		NewlyTx:         newlyTx,
		Time:            time.Now().Unix(),
	}
	sig, err := signer.Sign(msg.Id().Data)
	if err != nil {
		return nil, err
	}
	msg.Sig = sig
	return msg, nil
}

// Id 计算SignTxRequest的hashid
func (msg *SignTxRequest) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedSignMsgFields(hasher, "SignMsg", msg)
	return hasher.Sum(nil)
}

func (msg *SignTxRequest) MsgHash() string {
	return msg.Id().AsMapKey()
}

func UnmarshalSignTxRequest(bytes []byte) *SignTxRequest {
	rst := new(SignTxRequest)
	err := proto.Unmarshal(bytes, rst)
	assert.ErrorIsNil(err)
	return rst
}

func (msg *ChainTxIdMsg) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedChainTxIdMsgFields(hasher, "ChainTxIdMsg", msg)
	return hasher.Sum(nil)
}

func MakeChainTxIdMsg(txId string, signMsgId string, signer *crypto.SecureSigner) (*ChainTxIdMsg, error) {
	msg := &ChainTxIdMsg{
		TxId:      txId,
		SignMsgId: signMsgId,
	}
	sig, err := signer.Sign(msg.Id().Data)
	if err != nil {
		return nil, err
	}
	msg.Sig = sig
	return msg, nil
}

func MakeJoinRequest(host string, pubkey string, vote *Vote, signer *crypto.SecureSigner) (*JoinRequest, error) {
	msg := &JoinRequest{
		Host:   host,
		Pubkey: pubkey,
		Vote:   vote,
	}
	sig, err := signer.Sign(msg.Id().Data)
	if err != nil {
		return nil, err
	}
	msg.Sig = sig
	return msg, nil
}

func UnmarshalJoinRequest(bytes []byte) *JoinRequest {
	rst := new(JoinRequest)
	err := proto.Unmarshal(bytes, rst)
	assert.ErrorIsNil(err)
	return rst
}

func (msg *JoinRequest) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedJoinRequestFields(hasher, "JoinRequest", msg)
	return hasher.Sum(nil)
}

func MakeLeaveRequest(nodeId int32, msg string, signer *crypto.SecureSigner) (*LeaveRequest, error) {
	req := &LeaveRequest{
		NodeId: nodeId,
		Msg:    msg,
	}
	sig, err := signer.Sign(req.Id().Data)
	if err != nil {
		return nil, err
	}
	req.Sig = sig
	return req, nil
}

func UnmarshalLeaveRequest(bytes []byte) *LeaveRequest {
	rst := new(LeaveRequest)
	err := proto.Unmarshal(bytes, rst)
	assert.ErrorIsNil(err)
	return rst
}

func (msg *LeaveRequest) Id() *crypto.Digest256 {
	hasher := crypto.NewHasher256()
	feedLeaveRequestFields(hasher, "LeaveRequest", msg)
	return hasher.Sum(nil)
}

func (nl NodeList) GetPubkeys() []string {
	var pubkeys []string
	for _, node := range nl.NodeList {
		if node.IsNormal {
			pubkeys = append(pubkeys, node.Pubkey)
		}
	}
	return pubkeys
}
