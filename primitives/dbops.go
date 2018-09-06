package primitives

import (
	"encoding/json"
	"log"
	"math/big"
	"strconv"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/dgwdb"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/ofgp/ofgp-core/util"
	"github.com/ofgp/ofgp-core/util/assert"

	"github.com/golang/protobuf/proto"
	"github.com/syndtr/goleveldb/leveldb/iterator"
)

var (
	keyTerm                = []byte("NodeState_Term")
	keyLastTermAccuse      = []byte("NodeState_LastTermAccuse")
	keyWeakAccuses         = []byte("NodeState_WeakAccuses")
	keyFresh               = []byte("NodeState_Fresh")
	keyVotie               = []byte("NodeState_Votie")
	keyLastBchBlockHeader  = []byte("NodeState_LastBchBlockHeader")
	keyJoinNodeInfo        = []byte("JoinNodeInfo")
	keyLeaveNodeInfo       = []byte("LeaveNodeInfo")
	keyETHHeight           = []byte("ETHHeight")
	keyETHTxIndex          = []byte("ETHTxIndex")
	keyMultiSigSnapshot    = []byte("MS")
	keyCommitChainPrefix   = []byte("B")
	keyCommitHsByIdsPrefix = []byte("H")
	keyTxPrefix            = []byte("T")
	keyTxMapPrefix         = []byte("M")
	keySignMsgPrefix       = []byte("SM")
	keySignFailedPrefix    = []byte("SF")
	keySignedResultPrefix  = []byte("SR")
	keyHeightPrefix        = []byte("CH")
	keyProposalPrefix      = []byte("PP")
	// 保存accuse历史记录
	keyAccuseRecordsPrefix   = []byte("AR")
	keyClusterSnapshotPrefix = []byte("CS")
)

// InitDB 初始化leveldb数据
func InitDB(db *dgwdb.LDBDatabase, genesis *pb.BlockPack) {
	db.Delete(keyTerm)
	db.Delete(keyLastTermAccuse)
	db.Delete(keyWeakAccuses)
	db.Delete(keyFresh)
	db.Delete(keyVotie)
	db.Delete(keyLastBchBlockHeader)
	db.DeleteWithPrefix(keyCommitChainPrefix)
	db.DeleteWithPrefix(keyCommitHsByIdsPrefix)

	SetNodeTerm(db, 0)
	SetVotie(db, genesis.ToVotie())
	JustCommitIt(db, genesis)
}

// SetNodeTerm 设置节点的当前term
func SetNodeTerm(db *dgwdb.LDBDatabase, term int64) {
	assert.True(term >= 0)
	db.Put(keyTerm, util.I64ToBytes(term))
}

// GetNodeTerm 获取节点的当前term
func GetNodeTerm(db *dgwdb.LDBDatabase) (rst int64) {
	data, err := db.Get(keyTerm)
	assert.ErrorIsNil(err)
	term, err := util.BytesToI64(data)
	assert.ErrorIsNil(err)
	assert.True(term >= 0)
	return term
}

// SetLastTermAccuse 设置最近一次StrongAccuse
func SetLastTermAccuse(db *dgwdb.LDBDatabase, accuse *pb.StrongAccuse) {
	if accuse == nil {
		db.Delete(keyLastTermAccuse)
	} else {
		bytes, err := proto.Marshal(accuse)
		assert.ErrorIsNil(err)
		db.Put(keyLastTermAccuse, bytes)
	}
}

// GetLastTermAccuse 获取最近一次的accuse信息
func GetLastTermAccuse(db *dgwdb.LDBDatabase) *pb.StrongAccuse {
	data, err := db.Get(keyLastTermAccuse)
	if err != nil {
		return nil
	}
	return pb.UnmarshalStrongAccuse(data)
}

// SetWeakAccuses 保存当前的weakaccuse
func SetWeakAccuses(db *dgwdb.LDBDatabase, weakAccuses *pb.WeakAccuses) {
	if weakAccuses == nil {
		db.Delete(keyWeakAccuses)
	} else {
		bytes, err := proto.Marshal(weakAccuses)
		assert.ErrorIsNil(err)
		db.Put(keyWeakAccuses, bytes)
	}
}

// GetWeakAccuses 获取当前的weakaccuse
func GetWeakAccuses(db *dgwdb.LDBDatabase) *pb.WeakAccuses {
	data, err := db.Get(keyWeakAccuses)
	if err != nil {
		return pb.EmptyWeakAccuses()
	}
	return pb.UnmarshalWeakAccuses(data)
}

// SetFresh 保存当前共识中的block
func SetFresh(db *dgwdb.LDBDatabase, fresh *pb.BlockPack) {
	if fresh == nil {
		db.Delete(keyFresh)
	} else {
		bytes, err := proto.Marshal(fresh)
		assert.ErrorIsNil(err)
		db.Put(keyFresh, bytes)
	}
}

// GetFresh 获取当前共识中的block
func GetFresh(db *dgwdb.LDBDatabase) *pb.BlockPack {
	data, err := db.Get(keyFresh)
	if err != nil {
		return nil
	}
	return pb.UnmarshalBlockPack(data)
}

// HasFresh 判断当前有没有共识中的block
func HasFresh(db *dgwdb.LDBDatabase) bool {
	_, err := db.Get(keyFresh)
	return err == nil
}

// SetVotie 保存当前的投票信息
func SetVotie(db *dgwdb.LDBDatabase, votie *pb.Votie) {
	bytes, err := proto.Marshal(votie)
	assert.ErrorIsNil(err)
	db.Put(keyVotie, bytes)
}

// GetVotie 获取当前的投票信息
func GetVotie(db *dgwdb.LDBDatabase) *pb.Votie {
	data, err := db.Get(keyVotie)
	assert.ErrorIsNil(err)
	return pb.UnmarshalVotie(data)
}

// JustCommitIt 不做任何校验，直接保存区块
func JustCommitIt(db *dgwdb.LDBDatabase, blockPack *pb.BlockPack) {
	bpBytes, err := proto.Marshal(blockPack)
	assert.ErrorIsNil(err)
	hBytes := util.I64ToBytes(blockPack.Height())
	k := append(keyCommitChainPrefix, hBytes...)
	db.Put(k, bpBytes)
	k = append(keyCommitHsByIdsPrefix, blockPack.BlockId().Data...)
	db.Put(k, hBytes)
}

// GetCommitHeight 获取当前区块高度
func GetCommitHeight(db *dgwdb.LDBDatabase) int64 {
	iter := db.NewIteratorWithPrefix(keyCommitChainPrefix)
	defer iter.Release()
	if !iter.Last() {
		return -1
	}
	key := iter.Key()
	height, err := util.BytesToI64(key[len(keyCommitChainPrefix):])
	assert.ErrorIsNil(err)
	assert.True(height >= 0)
	return height
}

// GetCommitTop 获取当前最新区块
func GetCommitTop(db *dgwdb.LDBDatabase) *pb.BlockPack {
	iter := db.NewIteratorWithPrefix(keyCommitChainPrefix)
	defer iter.Release()
	if !iter.Last() {
		return nil
	}
	value := iter.Value()
	return pb.UnmarshalBlockPack(value)
}

// GetCommitByHeight 获取指定高度的区块
func GetCommitByHeight(db *dgwdb.LDBDatabase, height int64) *pb.BlockPack {
	assert.True(height >= 0)
	k := append(keyCommitChainPrefix, util.I64ToBytes(height)...)
	data, err := db.Get(k)
	if err != nil {
		return nil
	}
	return pb.UnmarshalBlockPack(data)
}

//根据height 范围获取区块
func GetCommitsByHeightSec(db *dgwdb.LDBDatabase, start, end int64) []*pb.BlockPack {
	if start >= end {
		return nil
	}
	startBytes := util.I64ToBytes(start)
	endBytes := util.I64ToBytes(end)
	s := append(keyCommitChainPrefix, startBytes...)
	e := append(keyCommitChainPrefix, endBytes...)
	iter := db.NewIteraterWithRange(s, e)
	bps := make([]*pb.BlockPack, 0)
	for iter.Next() {
		bytes := iter.Value()
		bp := pb.UnmarshalBlockPack(bytes)
		bps = append(bps, bp)
	}
	defer iter.Release()
	return bps
}

// IsCommitted check whether the blockId is already committed
func IsCommitted(db *dgwdb.LDBDatabase, blockId *crypto.Digest256) bool {
	k := append(keyCommitHsByIdsPrefix, blockId.Data...)
	_, err := db.Get(k)
	return err == nil
}

func GetCommitByID(db *dgwdb.LDBDatabase, blockID *crypto.Digest256) *pb.BlockPack {
	k := append(keyCommitHsByIdsPrefix, blockID.Data...)
	data, err := db.Get(k)
	if err != nil {
		log.Printf("get from db err:%v", err)
		return nil
	}
	height, err := util.BytesToI64(data)
	if err != nil {
		log.Printf("get block height by blcokid err:%v,data:%v", err, data)
		panic("get block height to int64 err")
	}
	return GetCommitByHeight(db, height)
}

// IsConnectingTop check whether the blockPack is right upon the top
func IsConnectingTop(db *dgwdb.LDBDatabase, blockPack *pb.BlockPack) bool {
	top := GetCommitTop(db)
	return blockPack.PrevBlockId().EqualTo(top.BlockId()) &&
		blockPack.Term() >= top.Term() &&
		blockPack.Height() == top.Height()+1
}

// SetTxLookupEntry 保存交易的索引信息
func SetTxLookupEntry(db *dgwdb.LDBDatabase, txId *crypto.Digest256, entry *pb.TxLookupEntry) {
	bytes, err := proto.Marshal(entry)
	assert.ErrorIsNil(err)
	k := append(keyTxPrefix, txId.Data...)
	db.Put(k, bytes)
}

// GetTxLookupEntry 获取指定交易的索引信息
func GetTxLookupEntry(db *dgwdb.LDBDatabase, txId *crypto.Digest256) *pb.TxLookupEntry {
	k := append(keyTxPrefix, txId.Data...)
	data, err := db.Get(k)
	if err != nil {
		return nil
	}
	return pb.UnmarshalTxLookupEntry(data)
}

// SetTxIdMap 保存公链交易id和网关交易id的映射
func SetTxIdMap(db *dgwdb.LDBDatabase, scTxId string, txId *crypto.Digest256) {
	k := append(keyTxMapPrefix, []byte(scTxId)...)
	db.Put(k, txId.Data)
}

// GetTxIdBySidechainTxId 根据公链交易id获取网关交易的id
func GetTxIdBySidechainTxId(db *dgwdb.LDBDatabase, scTxId string) *crypto.Digest256 {
	k := append(keyTxMapPrefix, []byte(scTxId)...)
	data, err := db.Get(k)
	if err != nil {
		return nil
	}
	return &crypto.Digest256{Data: data}
}

// SetSignMsg 保存SignTxRequest，方便后面做校验比对
func SetSignMsg(db *dgwdb.LDBDatabase, msg *pb.SignTxRequest, msgId string) {
	bytes, err := proto.Marshal(msg)
	assert.ErrorIsNil(err)
	k := append(keySignMsgPrefix, []byte(msgId)...)
	db.Put(k, bytes)
}

// GetSignMsg 获取签名时的上下文信息
func GetSignMsg(db *dgwdb.LDBDatabase, msgId string) *pb.SignTxRequest {
	k := append(keySignMsgPrefix, []byte(msgId)...)
	data, err := db.Get(k)
	if err != nil {
		return nil
	}
	return pb.UnmarshalSignTxRequest(data)
}

// DeleteSignMsg 清理签名信息
func DeleteSignMsg(db *dgwdb.LDBDatabase, msgId string) {
	k := append(keySignMsgPrefix, []byte(msgId)...)
	db.Delete(k)
}

// MarkFailedSignRecord 标记此term下的签名是否已经确认失败，需要重签
func MarkFailedSignRecord(db *dgwdb.LDBDatabase, msgId string, term int64) {
	k := append(keySignFailedPrefix, []byte(msgId+"_"+strconv.FormatInt(term, 10))...)
	db.Put(k, []byte("0"))
}

// IsSignFailed 判断此term下的签名是否已经确认失败
func IsSignFailed(db *dgwdb.LDBDatabase, msgId string, term int64) bool {
	k := append(keySignFailedPrefix, []byte(msgId+"_"+strconv.FormatInt(term, 10))...)
	data, _ := db.Get(k)
	return data != nil
}

// SetEthProposal 保存ETH多签交易的唯一标识
func SetEthProposal(db *dgwdb.LDBDatabase, proposal string) {
	k := append(keyProposalPrefix, []byte(proposal)...)
	db.Put(k, []byte{})
}

// IsProposalExist 检查指定的proposal是否存在
func IsProposalExist(db *dgwdb.LDBDatabase, proposal string) bool {
	k := append(keyProposalPrefix, []byte(proposal)...)
	_, err := db.Get(k)
	return err == nil
}

// DeleteEthProposal 删除指定的proposal。只在这笔交易确定失败了的时候才会删除
func DeleteEthProposal(db *dgwdb.LDBDatabase, proposal string) {
	k := append(keyProposalPrefix, []byte(proposal)...)
	db.Delete(k)
}

//func SetSignedStatistic(db *dgwdb.LDBDatabase, stat *pb.SignedStatistic) {
//	bytes, err := proto.Marshal(stat)
//	assert.ErrorIsNil(err)
//	k := append(keySignedResultPrefix, []byte(stat.SignedMsgId)...)
//	db.Put(k, bytes)
//}
//
//func GetSignedStatistic(db *dgwdb.LDBDatabase, msgId string) *pb.SignedStatistic {
//	k := append(keySignedResultPrefix, []byte(msgId)...)
//	data, err := db.Get(k)
//	if err != nil {
//		return nil
//	}
//	return pb.UnmarshalSignedStatistic(data)
//}

// SetCurrentHeight 保存当前公链监听到的高度
func SetCurrentHeight(db *dgwdb.LDBDatabase, chainType string, height int64) {
	k := append(keyHeightPrefix, []byte(chainType)...)
	db.Put(k, util.I64ToBytes(height))
}

// GetCurrentHeight 获取上次某条公链监听到的高度
func GetCurrentHeight(db *dgwdb.LDBDatabase, chainType string) int64 {
	k := append(keyHeightPrefix, []byte(chainType)...)
	data, err := db.Get(k)
	if err != nil {
		return 0
	}
	height, err := util.BytesToI64(data)
	if err != nil {
		return 0
	}
	return height
}

// SetJoinNodeInfo 保存JoinRequest信息
func SetJoinNodeInfo(db *dgwdb.LDBDatabase, msg *pb.JoinRequest) {
	bytes, err := proto.Marshal(msg)
	if err != nil {
		return
	}
	db.Put(keyJoinNodeInfo, bytes)
}

// GetJoinNodeInfo 获取之前保存的JoinRequest信息
func GetJoinNodeInfo(db *dgwdb.LDBDatabase) *pb.JoinRequest {
	data, err := db.Get(keyJoinNodeInfo)
	if err != nil {
		return nil
	}
	return pb.UnmarshalJoinRequest(data)
}

// DeleteJoinNodeInfo 删除JoinRequest信息
func DeleteJoinNodeInfo(db *dgwdb.LDBDatabase) {
	db.Delete(keyJoinNodeInfo)
}

// SetLeaveNodeInfo 保存LeaveRequest信息
func SetLeaveNodeInfo(db *dgwdb.LDBDatabase, msg *pb.LeaveRequest) {
	bytes, err := proto.Marshal(msg)
	if err != nil {
		return
	}
	db.Put(keyLeaveNodeInfo, bytes)
}

// GetLeaveNodeInfo 获取之前保存的LeaveRequest信息
func GetLeaveNodeInfo(db *dgwdb.LDBDatabase) *pb.LeaveRequest {
	data, err := db.Get(keyLeaveNodeInfo)
	if err != nil {
		return nil
	}
	return pb.UnmarshalLeaveRequest(data)
}

// DeleteLeaveNodeInfo 删除LeaveRequest信息
func DeleteLeaveNodeInfo(db *dgwdb.LDBDatabase) {
	db.Delete(keyLeaveNodeInfo)
}

// SetAccuseRecord 保存本节点发起的accuse记录
func SetAccuseRecord(db *dgwdb.LDBDatabase, term int64, localNodeId int32, leaderNodeId int32,
	accuseType int32, reason string) {
	timestamp := util.NowMs()
	tmp := &pb.AccuseRecord{
		Term:       term,
		LocalId:    localNodeId,
		LeaderId:   leaderNodeId,
		AccuseType: accuseType,
		Timestamp:  timestamp,
		Reason:     reason,
	}
	key := append(keyAccuseRecordsPrefix, []byte(strconv.FormatInt(timestamp, 10))...)
	bytes, err := proto.Marshal(tmp)
	if err != nil {
		return
	}
	db.Put(key, bytes)
}

// GetAccuseRecords 返回所有的accuse记录，返回的是一个iterator
func GetAccuseRecords(db *dgwdb.LDBDatabase) iterator.Iterator {
	return db.NewIteratorWithPrefix(keyAccuseRecordsPrefix)
}

// SetClusterSnapshot 保存相应多签地址对应的节点信息
func SetClusterSnapshot(db *dgwdb.LDBDatabase, scriptAddress string, snapshot cluster.Snapshot) {
	data, err := json.Marshal(snapshot)
	assert.ErrorIsNil(err)
	key := append(keyClusterSnapshotPrefix, []byte(scriptAddress)...)
	db.Put(key, data)
}

// GetClusterSnapshotIter 返回查询所有集群快照的迭代器
func GetClusterSnapshotIter(db *dgwdb.LDBDatabase) iterator.Iterator {
	return db.NewIteratorWithPrefix(keyClusterSnapshotPrefix)
}

// GetClusterSnapshot 返回指定多签地址对应的集群快照
func GetClusterSnapshot(db *dgwdb.LDBDatabase, scriptAddress string) *cluster.Snapshot {
	rst := &cluster.Snapshot{}
	key := append(keyClusterSnapshotPrefix, []byte(scriptAddress)...)
	data, err := db.Get(key)
	if err != nil {
		return nil
	}
	json.Unmarshal(data, rst)
	return rst
}

// SetMultiSigSnapshot 保存多签地址的快照
func SetMultiSigSnapshot(db *dgwdb.LDBDatabase, snapshot []cluster.MultiSigInfo) {
	data, err := json.Marshal(snapshot)
	assert.ErrorIsNil(err)
	db.Put(keyMultiSigSnapshot, data)
}

// GetMultiSigSnapshot 返回多签地址的快照
func GetMultiSigSnapshot(db *dgwdb.LDBDatabase) []cluster.MultiSigInfo {
	var rst []cluster.MultiSigInfo
	data, err := db.Get(keyMultiSigSnapshot)
	if err != nil {
		return rst
	}
	json.Unmarshal(data, &rst)
	return rst
}

func GetBlockByID(db *dgwdb.LDBDatabase, id []byte) *pb.BlockPack {
	key := append(keyCommitHsByIdsPrefix, id...)
	data, err := db.Get(key)
	if err != nil {
		log.Printf("get key:%v err:%v", key, err)
		return nil
	}
	height, err := util.BytesToI64(data)
	if err != nil {
		log.Printf("get height err:%v", err)
		return nil
	}
	return GetCommitByHeight(db, height)
}

// SetETHBlockHeight 保存ETH当前监听到的高度
func SetETHBlockHeight(db *dgwdb.LDBDatabase, height *big.Int) {
	db.Put(keyETHHeight, height.Bytes())
}

// GetETHBlockHeight 获取上次ETH监听到的高度
func GetETHBlockHeight(db *dgwdb.LDBDatabase) *big.Int {
	data, err := db.Get(keyETHHeight)
	if err != nil {
		return nil
	}
	res := &big.Int{}
	res.SetBytes(data)
	return res
}

// SetETHBlockTxIndex 保存当前ETH监听到的区块里面的哪一笔交易
func SetETHBlockTxIndex(db *dgwdb.LDBDatabase, index int) {
	data := strconv.Itoa(index)
	db.Put(keyETHTxIndex, []byte(data))
}

// GetETHBlockTxIndex 获取上次ETH监听到的区块里面的哪一笔交易
func GetETHBlockTxIndex(db *dgwdb.LDBDatabase) int {
	data, err := db.Get(keyETHTxIndex)
	if err != nil {
		return 0
	}
	res, err := strconv.Atoi(string(data))
	if err != nil {
		return 0
	}
	return res
}
