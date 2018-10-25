package node

import (
	"encoding/hex"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/crypto"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/spf13/viper"
	context "golang.org/x/net/context"
)

// Echo just for test echo
func (bn *BraftNode) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	if !checkEchoRequest(req) {
		nodeLogger.Error("echo req invalid")
		return nil, errInvalidRequest
	}
	nodeLogger.Debug("An echo request is received", "nonce", req.Nonce)
	rst := &pb.EchoResponse{
		Nonce: req.Nonce,
	}
	return rst, nil
}

// SyncUp 处理收到的同步请求
func (bn *BraftNode) SyncUp(ctx context.Context, msg *pb.SyncUpRequest) (*pb.SyncUpResponse, error) {
	if !checkSyncUpRequest(msg) {
		nodeLogger.Error(errInvalidRequest.Error())
		return new(pb.SyncUpResponse), errInvalidRequest
	}
	return bn.blockStore.GenSyncUpResponse(msg.BaseHeight, syncUpBatchSize, true), nil
}

func (bn *BraftNode) WatchSyncUp(ctx context.Context, msg *pb.SyncUpRequest) (*pb.SyncUpResponse, error) {
	if !checkSyncUpRequest(msg) {
		nodeLogger.Error(errInvalidRequest.Error())
		return new(pb.SyncUpResponse), errInvalidRequest
	}
	return bn.blockStore.GenSyncUpResponse(msg.BaseHeight, syncUpBatchSize, false), nil
}

// NotifyInitMsg 处理收到的initmsg请求, 新区块开始共识
func (bn *BraftNode) NotifyInitMsg(ctx context.Context, msg *pb.InitMsg) (*pb.Void, error) {
	nodeLogger.Debug("receive Init msg", "from", msg.NodeId)
	if !checkInit(msg, CheckFull) {
		nodeLogger.Error(errInvalidRequest.Error())
		return new(pb.Void), errInvalidRequest
	}
	bn.blockStore.HandleInitMsg(msg)
	return new(pb.Void), nil
}

// NotifyPrepareMsg 处理收到的preparemsg请求，新区块准备好了共识
func (bn *BraftNode) NotifyPrepareMsg(ctx context.Context, msg *pb.PrepareMsg) (*pb.Void, error) {
	nodeLogger.Debug("receive Prepare msg", "from", msg.NodeId)
	if !checkPrepare(msg) {
		nodeLogger.Error(errInvalidRequest.Error())
		return new(pb.Void), errInvalidRequest
	}
	bn.blockStore.HandlePrepareMsg(msg)
	return new(pb.Void), nil
}

// NotifyCommitMsg 处理收到的commitmsg请求，新区快达成共识
func (bn *BraftNode) NotifyCommitMsg(ctx context.Context, msg *pb.CommitMsg) (*pb.Void, error) {
	nodeLogger.Debug("receive Commit msg", "from", msg.NodeId)
	if !checkCommit(msg) {
		nodeLogger.Error(errInvalidRequest.Error())
		return new(pb.Void), errInvalidRequest
	}
	bn.blockStore.HandleCommitMsg(msg)
	return new(pb.Void), nil
}

// NotifyWeakAccuse 处理收到的weakaccuse请求，记录accuse个数，超过阈值则发起strong accuse
func (bn *BraftNode) NotifyWeakAccuse(ctx context.Context, msg *pb.WeakAccuse) (*pb.Void, error) {
	nodeLogger.Debug("receive WeakAccuse msg", "from", msg.NodeId)
	if !checkWeakAccuse(msg) {
		nodeLogger.Error(errInvalidRequest.Error())
		return new(pb.Void), errInvalidRequest
	}
	bn.blockStore.HandleWeakAccuse(msg)
	return new(pb.Void), nil
}

// NotifyStrongAccuse  处理收到的strongaccuse请求，更新term，重新选举leader
func (bn *BraftNode) NotifyStrongAccuse(ctx context.Context, msg *pb.StrongAccuse) (*pb.Void, error) {
	if !checkStrongAccuse(msg) {
		nodeLogger.Error(errInvalidRequest.Error())
		return new(pb.Void), errInvalidRequest
	}
	bn.blockStore.HandleStrongAccuse(msg)
	return new(pb.Void), nil
}

// NotifyVote 处理收到的投票请求，如果收到的票数超过阈值，自己会成为leader
func (bn *BraftNode) NotifyVote(ctx context.Context, msg *pb.Vote) (*pb.Void, error) {
	nodeLogger.Debug("Received notify vote", "from", msg.NodeId)
	if !checkVote(msg) {
		nodeLogger.Error(errInvalidRequest.Error(), "msg", msg.DebugString())
		return new(pb.Void), errInvalidRequest
	}
	bn.leader.AddVote(msg)
	nodeLogger.Debug("NotifyVote done")
	return new(pb.Void), nil
}

// NotifyTxs 暂时未使用
func (bn *BraftNode) NotifyTxs(ctx context.Context, msg *pb.Transactions) (*pb.Void, error) {
	return new(pb.Void), nil
}

// NotifySignTx 处理收到的加签请求
func (bn *BraftNode) NotifySignTx(ctx context.Context, msg *pb.SignTxRequest) (*pb.Void, error) {
	nodeLogger.Debug("Received notify sign tx request", "fromId", msg.NodeId)
	if !checkSignTxRequest(msg) {
		nodeLogger.Error(errInvalidRequest.Error())
		return nil, errInvalidRequest
	}
	bn.blockStore.HandleSignTx(msg)
	return new(pb.Void), nil
}

func (bn *BraftNode) NotifySignedResult(ctx context.Context, msg *pb.SignedResult) (*pb.Void, error) {
	nodeLogger.Debug("Received notify signed result", "fromId", msg.NodeId)
	if !checkSignedResult(msg) {
		nodeLogger.Error(errInvalidRequest.Error())
		return nil, errInvalidRequest
	}
	bn.SaveSignedResult(msg)
	return new(pb.Void), nil
}

// NotifyJoin 处理新节点加入的请求
func (bn *BraftNode) NotifyJoin(ctx context.Context, msg *pb.JoinRequest) (*pb.Void, error) {
	nodeLogger.Debug("Received notify join request")
	host := viper.GetString("DGW.new_node_host")
	pubkey := viper.GetString("DGW.new_node_pubkey")
	if !checkJoinRequest(msg, host, pubkey) {
		nodeLogger.Error(errInvalidRequest.Error())
		return nil, errInvalidRequest
	}
	bn.blockStore.HandleJoinRequest(msg)
	return new(pb.Void), nil
}

//join check是否同步完成
func (bn *BraftNode) NotifyJoinCheckSynced(ctx context.Context, msg *pb.JoinRequest) (*pb.JoinResponse, error) {
	nodeLogger.Debug("received NotifyJoinCheckSynced request")
	host := viper.GetString("DGW.new_node_host")
	pubkey := viper.GetString("DGW.new_node_pubkey")
	if !checkJoinRequest(msg, host, pubkey) {
		nodeLogger.Error(errInvalidRequest.Error())
		return nil, errInvalidRequest
	}
	nodeLogger.Debug("receive joinreq", "height", msg.GetVote().GetVotie().GetHeight())
	err := bn.blockStore.HandleJoinCheckSyncedRequest(msg)
	res := new(pb.JoinResponse)
	res.NodeID = bn.localNodeInfo.Id
	if err == nil {
		res.Synced = true
	}
	return res, nil
}

// NotifyLeave 处理节点的退出请求
func (bn *BraftNode) NotifyLeave(ctx context.Context, msg *pb.LeaveRequest) (*pb.Void, error) {
	nodeLogger.Debug("Received notify leave request")
	if !checkLeaveRequest(msg) {
		nodeLogger.Error(errInvalidRequest.Error())
		return nil, errInvalidRequest
	}
	bn.blockStore.HandleLeaveRequest(msg)
	return new(pb.Void), nil
}

// GetClusterNodes 处理获取集群节点列表的请求
func (bn *BraftNode) GetClusterNodes(ctx context.Context, msg *pb.Void) (*pb.NodeList, error) {
	nodeLogger.Debug("Received get cluster nodes request")
	rst := new(pb.NodeList)
	for _, node := range cluster.NodeList {
		rst.NodeList = append(rst.NodeList, &pb.NodeInfo{
			NodeId:   node.Id,
			Name:     node.Name,
			Pubkey:   hex.EncodeToString(node.PublicKey),
			Host:     node.Url,
			IsNormal: node.IsNormal,
		})
	}
	//设置当前quorumN
	rst.QuorumN = int32(cluster.QuorumN)
	oldMultisigs := bn.blockStore.GetMultiSigSnapshot()
	for _, multiSig := range oldMultisigs {
		rst.MultiSigInfoList = append(rst.MultiSigInfoList, &pb.MultiSigInfo{
			BtcAddress:      multiSig.BtcAddress,
			BtcRedeemScript: multiSig.BtcRedeemScript,
			BchAddress:      multiSig.BchAddress,
			BchRedeemScript: multiSig.BchRedeemScript,
		})
	}
	// 设置当前节点高度
	blockHeight := bn.blockStore.GetCommitHeight()
	rst.BlockHeight = blockHeight
	return rst, nil
}

func (bn *BraftNode) IsCommited(ctx context.Context, msg *crypto.Digest256) (*pb.IsCommitedResponse, error) {
	nodeLogger.Debug("Received isCommited request")
	if !checkDigest(msg) {
		return nil, errInvalidRequest
	}
	commited := bn.blockStore.IsCommitted(msg)
	res := &pb.IsCommitedResponse{
		Commited: commited,
	}
	return res, nil
}

func (bn *BraftNode) GetNodeRuntimeInfo(ctx context.Context, msg *pb.Void) (*pb.NodeRuntimeInfo, error) {
	nodeLogger.Debug("Received GetNodeRuntimeInfo")
	btchHeight := bn.btcWatcher.GetBlockNumber()
	bchHeight := bn.bchWatcher.GetBlockNumber()
	ethHeight := bn.ethWatcher.GetBlockNumber()
	data := &pb.NodeRuntimeInfo{
		NodeID:    bn.localNodeInfo.Id,
		BtcHeight: btchHeight,
		BchHeight: bchHeight,
		EthHeight: ethHeight,
		LeaderCnt: bn.leader.beComeLeaderCnt,
	}
	return data, nil
}
