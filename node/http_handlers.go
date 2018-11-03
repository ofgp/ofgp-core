package node

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	log "github.com/inconshreveable/log15"
	ew "github.com/ofgp/ethwatcher"
	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/distribution"
	dgwLog "github.com/ofgp/ofgp-core/log"
	"github.com/ofgp/ofgp-core/primitives"
	pb "github.com/ofgp/ofgp-core/proto"
)

var apiLog log.Logger

const firstBlockHeight = 0
const defaultCreateUsed = 10

func init() {
	apiLog = dgwLog.New("debug", "http_api")
}

func (node *BraftNode) GetNodeTerm() int64 {
	return node.blockStore.GetNodeTerm()
}

func (node *BraftNode) GetBlockHeight() int64 {
	return node.blockStore.GetCommitHeight()
}

func (node *BraftNode) GetBlockInfo(height int64) *pb.BlockInfo {
	blockPack := node.blockStore.GetCommitByHeight(height)
	return blockPack.BlockInfo()
}

func (node *BraftNode) GetTxBySidechainTxId(scTxId string) *primitives.TxQueryResult {
	return node.txStore.QueryTxInfoBySidechainId(scTxId)
}

// AddWatchedTx 手动添加交易
func (node *BraftNode) AddWatchedTx(tx *pb.WatchedTxInfo) error {
	if !node.txStore.HasWatchedTx(tx) {
		node.txStore.AddWatchedTx(tx)
	} else {
		return errors.New("tx already exist")
	}
	return nil
}

//blcokView for api
type BlockView struct {
	Height      int64     `json:"height"`
	ID          string    `json:"id"`
	PreID       string    `json:"pre_id"`
	TxCnt       int       `json:"tx_cnt"`
	Txs         []*TxView `json:"txs"`
	Time        int64     `json:"time"`         //unix 时间戳
	Size        int       `json:"size"`         //块大小
	CreatedUsed int64     `json:"created_used"` //块被创建花费的时间
	Miner       string    `json:"miner"`        //产生块的服务器
}

type TxView struct {
	FromTxHash  string   `json:"from_tx_hash"` //转出txhash
	ToTxHash    string   `json:"to_tx_hash"`   //转入txhash
	DGWTxHash   string   `json:"dgw_hash"`     //dgwtxhash
	From        string   `json:"from"`         //转出链
	To          string   `json:"to"`           //转入链
	Time        int64    `json:"time"`
	Block       string   `json:"block"`        //所在区块blockID
	BlockHeight int64    `json:"block_height"` //所在区块高度
	Amount      int64    `json:"amount"`
	ToAddrs     []string `json:"to_addrs"`
	FromFee     int64    `json:"from_fee"`
	DGWFee      int64    `json:"dgw_fee"`
	ToFee       int64    `json:"to_fee"`
	TokenCode   uint32   `json:"token_code"`
	AppCode     uint32   `json:"app_code"`
	FinalAmount int64    `json:"final_amount"` //扣除手续费后的金额
}

func getHexString(digest *crypto.Digest256) string {
	if digest == nil || len(digest.Data) == 0 {
		return ""
	}
	return hex.EncodeToString(digest.Data)
}

func (node *BraftNode) createTxView(blockID string, height int64, tx *pb.Transaction) *TxView {
	if tx == nil {
		apiLog.Error(fmt.Sprintf("block:%s transaciton is nil", blockID))
		return nil
	}
	watchedTx := tx.GetWatchedTx()
	if watchedTx == nil {
		apiLog.Error(fmt.Sprintf("block:%s has no watched tx", blockID))
		return nil
	}
	addrList := watchedTx.GetRechargeList()
	addrs := make([]string, 0)
	for _, addr := range addrList {
		addrs = append(addrs, addr.Address)
	}

	var tokenCode, appCode uint32
	if watchedTx.From == "eth" || watchedTx.From == "xin" { //熔币
		tokenCode, appCode = watchedTx.TokenTo, watchedTx.TokenFrom
	} else { //铸币
		tokenCode, appCode = watchedTx.TokenFrom, watchedTx.TokenTo
	}
	txView := &TxView{
		FromTxHash:  watchedTx.GetTxid(),
		DGWTxHash:   getHexString(tx.GetId()),
		ToTxHash:    tx.NewlyTxId,
		From:        watchedTx.From,
		To:          watchedTx.To,
		Block:       blockID,
		BlockHeight: height,
		Amount:      watchedTx.Amount,
		FromFee:     watchedTx.GetFee(),
		ToAddrs:     addrs,
		Time:        tx.Time,
		TokenCode:   tokenCode,
		AppCode:     appCode,
		FinalAmount: tx.Amount,
		DGWFee:      watchedTx.Amount - tx.Amount,
	}
	return txView
}
func (node *BraftNode) createBlockView(createdUsed int64, blockPack *pb.BlockPack) *BlockView {
	block := blockPack.Block()
	var txViews []*TxView
	if block == nil {
		return nil
	}
	var height int64
	var nodeID int32
	height = blockPack.Height()
	if blockPack.GetInit() != nil {
		nodeID = blockPack.GetInit().GetNodeId()
	}
	peerNode := node.peerManager.GetNode(nodeID)
	var miner string
	if peerNode != nil {
		miner = peerNode.Name
	}
	txs := block.Txs
	txViews = make([]*TxView, 0)
	blockID := getHexString(block.GetId())
	for _, tx := range txs {
		txView := node.createTxView(blockID, height, tx)
		if txView != nil {
			txViews = append(txViews, txView)
		} else {
			apiLog.Error("create txview err", "transaction", tx)
		}
	}

	bw := &BlockView{
		Height:      height,
		ID:          getHexString(block.Id),
		PreID:       getHexString(block.PrevBlockId),
		TxCnt:       len(txViews),
		Txs:         txViews,
		Time:        int64(block.TimestampMs / 1000),
		Size:        blockPack.XXX_Size(),
		CreatedUsed: createdUsed,
		Miner:       miner,
	}
	return bw
}

//获取最新区块
func (node *BraftNode) GetBlockCurrent() *BlockView {
	curHeight := node.blockStore.GetCommitHeight()
	return node.GetBlockBytHeight(curHeight)
}

//获取区块 start 开始高度 end结束高度[start,end)
func (node *BraftNode) GetBlocks(start, end int64) []*BlockView {
	if end <= start {
		apiLog.Error(fmt.Sprintf("end:%d is less than start:%d", start, end))
		return nil
	}
	var beiginFromFirstBlock bool
	if start < firstBlockHeight {
		apiLog.Error("start < firstBlockHeight")
		return nil
	}
	if start == firstBlockHeight {
		beiginFromFirstBlock = true
	}
	if start > firstBlockHeight { //大于创世块
		start = start - 1
	}

	blockPacks := node.blockStore.GetCommitsByHeightSec(start, end)
	size := len(blockPacks)
	if size == 0 {
		return nil
	}
	blcokViews := make([]*BlockView, 0)
	if beiginFromFirstBlock { //==创世块
		blockView := node.createBlockView(0, blockPacks[0])
		blcokViews = append(blcokViews, blockView)
	}
	pre := blockPacks[0]

	for i := 1; i < size; i++ {
		blcokPack := blockPacks[i]
		if blcokPack == nil {
			continue
		}
		var createdUsed int64
		if pre.TimestampMs() == 0 {
			createdUsed = defaultCreateUsed
		} else {
			createdUsed = blcokPack.TimestampMs()/1000 - pre.TimestampMs()/1000
		}
		blockView := node.createBlockView(createdUsed, blcokPack)
		blcokViews = append(blcokViews, blockView)
		pre = blcokPack
	}

	return blcokViews
}

//根据高度获取block
func (node *BraftNode) GetBlockBytHeight(height int64) *BlockView {
	views := node.GetBlocks(height, height+1)
	if len(views) == 0 {
		return nil
	}
	return views[0]
}
func (node *BraftNode) GetBlockByID(id string) (*BlockView, error) {
	idreal, err := hex.DecodeString(id)
	if err != nil {
		return nil, err
	}
	block := node.blockStore.GetBlockByID(idreal)
	var preBlock *pb.BlockPack
	if block.PrevBlockId() != nil && len(block.PrevBlockId().Data) > 0 {
		preBlock = node.blockStore.GetBlockByID(block.PrevBlockId().Data)
	}
	var createdUsed int64
	if preBlock != nil {
		createdUsed = block.TimestampMs()/1000 - preBlock.TimestampMs()/1000
	}
	blockView := node.createBlockView(createdUsed, block)
	return blockView, nil
}

//根据tx_id查询 transaction 不同链的tx_id用相同的pre存储
func (node *BraftNode) GetTransacitonByTxID(txID string) *TxView {
	txQueryResult := node.txStore.GetTx(txID)
	if txQueryResult == nil {
		return nil
	}
	tx := txQueryResult.Tx
	blockID := getHexString(txQueryResult.BlockID)
	txView := node.createTxView(blockID, txQueryResult.Height, tx)
	return txView
}

// FakeCommitBlock 人工写区块，仅做测试用
func (node *BraftNode) FakeCommitBlock(blockPack *pb.BlockPack) {
	node.blockStore.JustCommitIt(blockPack)
	node.txStore.OnNewBlockCommitted(blockPack)
}

//节点数据
type NodeView struct {
	id        int
	IP        string `json:"ip"`
	HostName  string `json:"host_name"`
	IsLeader  bool   `json:"is_leader"`
	IsOnline  bool   `json:"is_online"`
	FiredCnt  int32  `json:"fired_cnt"` //被替换掉leader的次数
	EthHeight int64  `json:"eth_height"`
	BchHeight int64  `json:"bch_height"`
	BtcHeight int64  `json:"btc_height"`
}

func (node *BraftNode) GetNodes() []NodeView {
	nodeViews := make([]NodeView, 0)
	term := node.GetNodeTerm()
	leaderID := cluster.LeaderNodeOfTerm(term)
	apiLog.Error("leader id is", "leaderID", leaderID, "term is ", term)
	nodeRuntimeInfos := node.peerManager.GetNodeRuntimeInfos()
	for _, node := range cluster.NodeList {
		var isLeader bool
		ip, _ := getHostAndPort(node.Url)
		if leaderID == node.Id {
			isLeader = true
		}
		var firedCnt int32
		ethH, btcH, bchH, LeaderCnt := getHeightAndLeaderCnt(node.Id, nodeRuntimeInfos)
		if isLeader && LeaderCnt > 0 {
			firedCnt = LeaderCnt - 1
		} else {
			firedCnt = LeaderCnt
		}
		nodeView := NodeView{
			IP:        ip,
			HostName:  node.Name,
			IsLeader:  isLeader,
			IsOnline:  node.IsNormal,
			EthHeight: ethH,
			BtcHeight: btcH,
			BchHeight: bchH,
			FiredCnt:  firedCnt,
		}
		nodeViews = append(nodeViews, nodeView)
	}
	sort.Slice(nodeViews, func(i, j int) bool {
		return nodeViews[i].id < nodeViews[j].id
	})
	return nodeViews
}

type ChainRegInfo struct {
	NewChain    string `json:"new_chain"`
	TargetChain string `json:"target_chain"`
}

type ChainRegID struct {
	ChainID uint32 `json:"chain_id"`
}

type TokenRegInfo struct {
	ContractAddr string `json:"contract_addr"`
	Chain        string `json:"chain"`
	ReceptChain  uint32 `json:"recept_chain"`
	ReceptToken  uint32 `json:"recept_token"`
}

type TokenRegID struct {
	TokenID uint32 `json:"token_id"`
}

// ChainRegister 新链注册
func (node *BraftNode) ChainRegister(regInfo *ChainRegInfo) {
	if regInfo.TargetChain == "eth" {
		proposal := strings.Join([]string{"CR", regInfo.TargetChain, regInfo.NewChain}, "_")
		_, err := node.ethWatcher.GatewayTransaction(node.signer.PubKeyHex, node.signer.PubkeyHash,
			ew.VOTE_METHOD_ADDCHAIN, regInfo.NewChain, proposal)
		if err != nil {
			nodeLogger.Error("register new chain failed", "err", err, "newchain", regInfo.NewChain)
		} else {
			nodeLogger.Debug("register new chain success", "newchain", regInfo.NewChain)
		}
	}
}

// GetChainRegisterID 查询链注册结果，如果成功，返回chainID，否则返回0
func (node *BraftNode) GetChainRegisterID(newChain string, targetChain string) *ChainRegID {
	if targetChain == "eth" {
		chainID := node.ethWatcher.GetChainCode(newChain)
		return &ChainRegID{ChainID: chainID}
	}
	return nil
}

// TokenRegister 新token合约注册
func (node *BraftNode) TokenRegister(regInfo *TokenRegInfo) {
	if regInfo.Chain == "eth" {
		// 调用ETH合约接口注册新的token合约, proposal就使用contractaddr
		addr := ew.HexToAddress(regInfo.ContractAddr)
		_, err := node.ethWatcher.GatewayTransaction(node.signer.PubKeyHex, node.signer.PubkeyHash, ew.VOTE_METHOD_ADDAPP,
			addr, regInfo.ReceptChain, regInfo.ReceptToken, "TR_"+regInfo.ContractAddr)
		if err != nil {
			nodeLogger.Error("register new token failed", "err", err, "contractaddr", regInfo.ContractAddr)
		} else {
			nodeLogger.Debug("register new token success", "contractaddr", regInfo.ContractAddr)
		}
	}
}

// GetTokenRegisterID 查询token注册结果，如果成功，则返回tokenID, 否则返回0
func (node *BraftNode) GetTokenRegisterID(chain string, contractAddr string) *TokenRegID {
	if chain == "eth" {
		tokenID := node.ethWatcher.GetAppCode(contractAddr)
		return &TokenRegID{TokenID: tokenID}
	}
	return nil
}

// ManualMintRequest 手工铸币结构
type ManualMintRequest struct {
	Amount   int64  `json:"amount"`
	Address  string `json:"address"`
	Proposal string `json:"proposal"`
	Chain    uint32 `json:"chain"`
	Token    uint32 `json:"token"`
}

// ManualMint 手工铸币
func (node *BraftNode) ManualMint(mintInfo *ManualMintRequest) {
	addr := ew.HexToAddress(mintInfo.Address)
	_, err := node.ethWatcher.GatewayTransaction(node.signer.PubKeyHex, node.signer.PubkeyHash, ew.VOTE_METHOD_MINT,
		mintInfo.Token, uint64(mintInfo.Amount), addr, "MM_"+mintInfo.Proposal)
	if err != nil {
		nodeLogger.Error("manual mint failed", "err", err)
	} else {
		nodeLogger.Debug("manual mint success")
	}
}

func getHostAndPort(url string) (host, port string) {
	if url == "" {
		return
	}
	strs := strings.Split(url, ":")
	if len(strs) >= 2 {
		host = strs[0]
		port = strs[1]
	}
	return
}

func getHeightAndLeaderCnt(nodeID int32,
	apiDatas map[int32]*pb.NodeRuntimeInfo) (ethHeight, btcHeight, bchHeight int64, leaderCnt int32) {
	apiData := apiDatas[nodeID]
	if apiData == nil {
		return
	}
	return apiData.EthHeight, apiData.BtcHeight, apiData.BchHeight, apiData.LeaderCnt
}

// GetMinBCHMintAmount 返回BCH链铸币的最小金额
func (node *BraftNode) GetMinBCHMintAmount() int64 {
	return node.minBCHMintAmount
}

// GetMinBTCMintAmount 返回BCH链铸币的最小金额
func (node *BraftNode) GetMinBTCMintAmount() int64 {
	return node.minBTCMintAmount
}

// GetMinBurnAmount 返回熔币的最小金额
func (node *BraftNode) GetMinBurnAmount() int64 {
	return node.minBurnAmount
}

// GetMintFeeRate 返回铸币的手续费
func (node *BraftNode) GetMintFeeRate() int64 {
	return node.mintFeeRate
}

// GetBurnFeeRate 返回熔币的手续费
func (node *BraftNode) GetBurnFeeRate() int64 {
	return node.burnFeeRate
}

// AddProposal 新增一个分配提案
func (node *BraftNode) AddProposal(p *distribution.Proposal) bool {
	return node.proposalManager.AddProposal(p)
}

// GetProposal 获取指定的提案
func (node *BraftNode) GetProposal(proposalID string) *distribution.DistributionInfo {
	return node.proposalManager.GetProposal(proposalID)
}

// GetAllProposal 获取所有的提案
func (node *BraftNode) GetAllProposal() []*distribution.DistributionInfo {
	return node.proposalManager.ListProposal()
}

// DeleteProposal 删除指定的提案，注意只有处于new或者执行失败的情况下，提案才会被删除
func (node *BraftNode) DeleteProposal(proposalID string) bool {
	return node.proposalManager.DeleteProposal(proposalID)
}

// ExecuteProposal 执行指定的提案
func (node *BraftNode) ExecuteProposal(proposalID string) {
	node.proposalManager.ExecuteProposal(proposalID)
}

// AddSideTx 添加侧链tx
func (node *BraftNode) AddSideTx(txID string, chain string) error {
	var watchedTx *pb.WatchedTxInfo
	switch chain {
	case "bch":
		subTx := node.bchWatcher.GetTxByHash(txID)

		if subTx != nil {
			if subTx.SubTx != nil {
				watchedTx = pb.BtcToPbTx(subTx.SubTx)
			} else {
				return errors.New("bch sub tx nil")
			}
		} else {
			return errors.New("get bch tx nil")
		}
	case "btc":
		subTx := node.btcWatcher.GetTxByHash(txID)
		if subTx != nil {
			if subTx.SubTx != nil {
				watchedTx = pb.BtcToPbTx(subTx.SubTx)
			} else {
				return errors.New("btc sub tx nil")
			}
		} else {
			return errors.New("get btc tx nil")
		}
	case "eth":
		fmt.Printf("add watchedtx txid:%s\n", txID)
		event, err := node.ethWatcher.GetEventByHash(txID)
		if err == nil {
			if ew.TOKEN_METHOD_BURN == event.Method {
				if (event.Events & ew.TX_STATUS_FAILED) != 0 {
					nodeLogger.Debug("burn tx is failed in contract")
					return errors.New("burn tx is failed in contract")
				}
				burnData := event.ExtraData.(*ew.ExtraBurnData)
				nodeLogger.Debug("receive eth burn", "tx", burnData.ScTxid)
				if burnData.Amount < uint64(node.minBurnAmount) {
					nodeLogger.Debug("amount is less than minimal amount", "sctxid", burnData.ScTxid)
					return errors.New("amount is less than minimal amount")
				}
				watchedTx = pb.EthToPbTx(burnData)
			}
		} else {
			return errors.New("get eth tx nil")
		}
	case "xin":
		event, err := node.xinWatcher.GetEventByTxid(txID)
		// nodeLogger.Debug("get xin event", "account", event.Account, "name", event.Name, "memo", event.Memo, "amount", event.Amount, "block", event.BlockNum, "index", event.Index)
		if err != nil {
			nodeLogger.Error("get xin err", "err", err, "scTxID", txID)
			return err
		}
		if event != nil {
			watchedTx = pb.XINToPbTx(event)
			if watchedTx == nil {
				return errors.New("xin event to watched err")
			}
		} else {
			return errors.New("get xin tx nil")
		}
	}
	err := node.txStore.AddWatchedInfo(watchedTx)
	return err
}
