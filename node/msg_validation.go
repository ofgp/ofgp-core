package node

import (
	"dgateway/cluster"
	"dgateway/crypto"
	"dgateway/primitives"
	pb "dgateway/proto"
	"encoding/hex"
)

type CheckMode int

const (
	CheckFull CheckMode = iota
	CheckLite
)

func checkTerm(term int64) bool {
	return term >= 0
}

func checkHeight(height int64) bool {
	return height >= 0
}

func checkNodeId(id int32) bool {
	return id >= 0 && id < int32(len(cluster.NodeList))
}

func checkDigest(d *crypto.Digest256) bool {
	return d != nil && len(d.Data) == 256/8
}

// host ip:port
func checkHost(host string) bool {
	return len(host) >= 0 && len(host) < 22
}

func checkBchBlockHeader(header *pb.BchBlockHeader) bool {
	return header.Height >= 0 && checkDigest(header.BlockId) && checkDigest(header.PrevId)
}

func checkBlock(block *pb.Block) bool {
	if block == nil {
		return false
	}
	if block.Type == pb.Block_GENESIS {
		if !(block.TimestampMs == 0 && block.PrevBlockId == nil && len(block.Txs) == 0 &&
			checkBchBlockHeader(block.BchBlockHeader) && block.Reconfig == nil) {
			return false
		}
		block.UpdateBlockId()
		return true
	} else if block.Type == pb.Block_TXS || block.Type == pb.Block_RECONFIG {
		if !checkDigest(block.PrevBlockId) || block.BchBlockHeader != nil {
			return false
		}
		block.UpdateBlockId()
		return true
	} else if block.Type == pb.Block_BCH {
		if !(checkDigest(block.PrevBlockId) && checkBchBlockHeader(block.BchBlockHeader) &&
			block.Reconfig == nil) {
			return false
		}
		block.UpdateBlockId()
		return true
	} else {
		return false
	}
}

//check initMsg
func checkInitMsg(msg *pb.InitMsg) bool {
	if msg == nil || !checkTerm(msg.Term) || !checkHeight(msg.Height) ||
		!checkBlock(msg.Block) || !checkNodeId(msg.NodeId) {
		nodeLogger.Warn("initmsg data is invalid", "height", msg.Height)
		return false
	}
	for nodeID, vote := range msg.Votes {
		if !checkNodeId(nodeID) || vote == nil || vote.Term != msg.Term || vote.NodeId != nodeID {
			nodeLogger.Warn("initmsg vote data is invalid", "height", msg.Height)
			return false
		}
		if !checkVote(vote) {
			nodeLogger.Warn("vote check fail", "height", msg.Height)
			return false
		}
	}
	ok := crypto.Verify(cluster.NodeList[msg.NodeId].PublicKey, msg.Id(), msg.Sig)
	if !ok {
		nodeLogger.Error("veryfiy fail", "height", msg.Height)
	}
	return ok
}
func checkInit(msg *pb.InitMsg, mode CheckMode) bool {
	if msg == nil || !checkTerm(msg.Term) || !checkHeight(msg.Height) ||
		!checkBlock(msg.Block) || !checkNodeId(msg.NodeId) {
		return false
	}

	if len(msg.Votes) >= cluster.QuorumN {
		if mode == CheckFull {
			for nodeId, vote := range msg.Votes {
				if !checkNodeId(nodeId) || vote == nil || vote.Term != msg.Term || vote.NodeId != nodeId {
					return false
				}
				if !checkVote(vote) {
					return false
				}
			}
		}
	} else {
		msg.Votes = nil
	}

	if mode == CheckLite {
		return true
	}
	return crypto.Verify(cluster.NodeList[msg.NodeId].PublicKey, msg.Id(), msg.Sig)
}

func checkVote(vote *pb.Vote) bool {
	//return vote != nil && checkTerm(vote.Term) && checkNodeId(vote.NodeId) &&
	//	vote.Votie.Term <= vote.Term && checkVotie(vote.Votie) &&
	//	crypto.Verify(cluster.NodeList[vote.NodeId].PublicKey, vote.Id(), vote.Sig)
	if !(vote != nil && checkTerm(vote.Term) && checkNodeId(vote.NodeId) &&
		vote.Votie.Term <= vote.Term && checkVotie(vote.Votie)) {
		nodeLogger.Debug("checkVote content wrong")
		return false
	}
	if crypto.Verify(cluster.NodeList[vote.NodeId].PublicKey, vote.Id(), vote.Sig) {
		return true
	}
	nodeLogger.Debug("checkVote sig verify failed")
	return false
}

func checkVotie(v *pb.Votie) bool {
	if v == nil || !checkTerm(v.Term) || !checkHeight(v.Height) || !checkBlock(v.Block) {
		nodeLogger.Debug("checkVotie base check failed")
		return false
	}

	for nodeId, prepare := range v.Prepares {
		if !checkPrepare(prepare) || nodeId != prepare.NodeId {
			nodeLogger.Warn("checkVotie check prepare failed", "nodeId", nodeId, "prepare", prepare.DebugString())
			return false
		}
		if prepare.Term != v.Term || prepare.Height != v.Height || !v.Block.Id.EqualTo(prepare.BlockId) {
			nodeLogger.Warn("checkVotie return false", "pterm", prepare.Term, "vterm", v.Term,
				"pheight", prepare.Height, "vheight", v.Height, "isequal", v.Block.Id.EqualTo(prepare.BlockId))
			return false
		}
	}

	for nodeId, commit := range v.Commits {
		if !checkCommit(commit) || nodeId != commit.NodeId {
			nodeLogger.Debug("checkVotie check commits failed")
			return false
		}
		if commit.Term != v.Term || commit.Height != v.Height || !v.Block.Id.EqualTo(commit.BlockId) {
			nodeLogger.Debug("checkVotie return false", "cterm", commit.Term, "vterm", v.Term,
				"cheight", commit.Height, "vheight", v.Height, "isequal", v.Block.Id.EqualTo(commit.BlockId))
			return false
		}
	}

	if v.Height == 0 {
		// TODO: 暂时没有比较创世区块id
		return v.Term == 0 && len(v.Prepares) == 0 && len(v.Commits) == 0
	}
	return len(v.Prepares) >= cluster.QuorumN || len(v.Commits) >= cluster.QuorumN
}

func checkPrepare(msg *pb.PrepareMsg) bool {
	return msg != nil && checkTerm(msg.Term) && checkHeight(msg.Height) &&
		checkDigest(msg.BlockId) && checkNodeId(msg.NodeId) &&
		crypto.Verify(cluster.NodeList[msg.NodeId].PublicKey, msg.Id(), msg.Sig)
}

func checkCommit(msg *pb.CommitMsg) bool {
	return msg != nil && checkTerm(msg.Term) && checkHeight(msg.Height) &&
		checkDigest(msg.BlockId) && checkNodeId(msg.NodeId) &&
		crypto.Verify(cluster.NodeList[msg.NodeId].PublicKey, msg.Id(), msg.Sig)
}

func checkWeakAccuse(msg *pb.WeakAccuse) bool {
	return msg != nil && checkTerm(msg.Term) && checkNodeId(msg.NodeId) &&
		crypto.Verify(cluster.NodeList[msg.NodeId].PublicKey, msg.Id(), msg.Sig)
}

func checkStrongAccuse(msg *pb.StrongAccuse) bool {
	if msg == nil || len(msg.WeakAccuses) < cluster.AccuseQuorumN {
		return false
	}

	for nodeId, wa := range msg.WeakAccuses {
		if !checkWeakAccuse(wa) || nodeId != wa.NodeId {
			return false
		}
	}
	return true
}

func checkInitedBlockPack(bp *pb.BlockPack, mode CheckMode) bool {
	if bp == nil {
		return false
	}

	init := bp.Init
	if !checkInit(init, mode) {
		nodeLogger.Error("check init failed")
		return false
	}

	if mode == CheckLite {
		return true
	}

	var (
		blockTerm   = init.Term
		blockHeight = init.Height
		blockId     = init.Block.Id
	)

	for nodeId, prepare := range bp.Prepares {
		if !checkPrepare(prepare) || nodeId != prepare.NodeId {
			return false
		}
		if prepare.Term != blockTerm || prepare.Height != blockHeight ||
			!prepare.BlockId.EqualTo(blockId) {
			return false
		}
	}

	for nodeId, commit := range bp.Commits {
		if !checkCommit(commit) || nodeId != commit.NodeId {
			return false
		}
		if commit.Term != blockTerm || commit.Height != blockHeight ||
			!commit.BlockId.EqualTo(blockId) {
			return false
		}
	}

	return true
}

//checkPrepares
func checkPrepares(blockTerm, blockHeight int64, blockID *crypto.Digest256, prepares map[int32]*pb.PrepareMsg) bool {
	if len(prepares) == 0 {
		nodeLogger.Warn("prepares's cnt is zero", "height", blockHeight)
		return false
	}
	for nodeID, prepare := range prepares {
		if !checkPrepare(prepare) || nodeID != prepare.NodeId {
			nodeLogger.Warn("prepare check fail", "height", blockHeight)
			return false
		}
		if prepare.Term != blockTerm || prepare.Height != blockHeight ||
			!prepare.BlockId.EqualTo(blockID) {
			nodeLogger.Warn("prepares's data is not equal block", "height", blockHeight)
			return false
		}
	}
	return true
}

//checkcommits
func checkCommits(blockTerm, blockHeight int64, blockID *crypto.Digest256, commits map[int32]*pb.CommitMsg) bool {
	if len(commits) == 0 {
		nodeLogger.Warn("commits'cnt is zero", "height", blockHeight)
		return false
	}
	for nodeID, commit := range commits {
		if !checkCommit(commit) || nodeID != commit.NodeId {
			nodeLogger.Warn("commit check fail", "height", blockHeight)
			return false
		}
		if commit.Term != blockTerm || commit.Height != blockHeight ||
			!commit.BlockId.EqualTo(blockID) {
			nodeLogger.Warn("commits'data is not equal to block", "height", blockHeight)
			return false
		}
	}
	return true
}

func checkEchoRequest(req *pb.EchoRequest) bool {
	return req != nil && req.Nonce >= 0
}

func checkSyncUpRequest(req *pb.SyncUpRequest) bool {
	return req != nil && req.BaseHeight >= 0
}
func checkSyncUpByHashRequest(req *pb.SyncUpByHashRequest) bool {
	if req == nil || len(req.Locator) == 0 {
		return false
	}
	if !req.Locator[len(req.Locator)-1].EqualTo(primitives.GenesisBlockPack.BlockId()) {
		return false
	}
	return true
}

func checkSyncUpResponse(msg *pb.SyncUpResponse) bool {
	if msg == nil {
		nodeLogger.Error("check syncup response failed, msg is nil")
		return false
	}

	if len(msg.Commits) > 0 {
		var prevHeight int64 = -1
		var prevTerm int64 = -1
		var prevBlockId *crypto.Digest256
		var checkMode = CheckLite

		for idx, commit := range msg.Commits {
			if idx == len(msg.Commits)-1 {
				checkMode = CheckFull
			}
			if !checkInitedBlockPack(commit, checkMode) {
				nodeLogger.Error("check inited blockpack failed")
				return false
			}
			if prevHeight != -1 {
				if commit.Height() != prevHeight+1 || !commit.PrevBlockId().EqualTo(prevBlockId) ||
					prevTerm > commit.Term() {
					return false
				}
			}

			prevHeight = commit.Height()
			prevTerm = commit.Term()
			prevBlockId = commit.BlockId()

			// TODO 这里暂时把更多的检查去掉了
			//if checkMode == CheckFull && len(commit.Commits) < cluster.QuorumN {
			//	nodeLogger.Error("bbbbbbbb")
			//	return false
			//}
		}
	}

	if msg.Fresh != nil {
		if !checkInitedBlockPack(msg.Fresh, CheckFull) {
			return false
		}
	}

	if msg.StrongAccuse != nil {
		if !checkStrongAccuse(msg.StrongAccuse) {
			return false
		}
	}
	return true
}

func checkSignTxRequest(req *pb.SignTxRequest) bool {
	return req != nil && checkTerm(req.Term) && checkNodeId(req.NodeId) &&
		crypto.Verify(cluster.NodeList[req.NodeId].PublicKey, req.Id(), req.Sig)
}

func checkSignedResult(msg *pb.SignedResult) bool {
	return msg != nil && checkNodeId(msg.NodeId) && len(msg.TxId) > 0 && len(msg.To) > 0 &&
		checkTerm(msg.Term) && crypto.Verify(cluster.NodeList[msg.NodeId].PublicKey, msg.Id(), msg.Sig)
}

func checkChainTxIdMsg(msg *pb.ChainTxIdMsg) bool {
	if msg == nil {
		return false
	}
	return crypto.Verify(cluster.NodeList[msg.NodeId].PublicKey, msg.Id(), msg.Sig)
}

func checkJoinVote(vote *pb.Vote, pubkey []byte) bool {
	if !(vote != nil && checkTerm(vote.Term) && vote.NodeId == int32(cluster.TotalNodeCount) &&
		vote.Votie.Term <= vote.Term && checkVotie(vote.Votie)) {
		nodeLogger.Warn("checkVote content wrong", "clustersize", len(cluster.NodeList), "vote", vote.DebugString())
		return false
	}
	if crypto.Verify(pubkey, vote.Id(), vote.Sig) {
		return true
	}
	nodeLogger.Warn("checkVote sig verify failed")
	return false
}

func checkJoinRequest(req *pb.JoinRequest, host string, pubkeyHex string) bool {
	if req == nil || !checkHost(req.Host) || req.Host != host || req.Pubkey != pubkeyHex {
		nodeLogger.Debug("check joinrequest failed", "host", req.Host)
		return false
	}
	if req.GetVote() == nil || req.GetVote().GetVotie() == nil {
		nodeLogger.Error("vote or votie is nil", "joinreq", req.Vote)
		return false
	}
	pubkey, err := hex.DecodeString(req.Pubkey)
	if err != nil {
		nodeLogger.Debug("check joinrequest failed", "err", err)
		return false
	}
	return checkJoinVote(req.Vote, pubkey) && crypto.Verify(pubkey, req.Id(), req.Sig)
}

func checkLeaveRequest(req *pb.LeaveRequest) bool {
	return req != nil && checkNodeId(req.NodeId) &&
		crypto.Verify(cluster.NodeList[req.NodeId].PublicKey, req.Id(), req.Sig)
}
