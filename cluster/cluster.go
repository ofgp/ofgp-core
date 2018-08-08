package cluster

import (
	"bytes"
)

// NodeInfo 节点信息
type NodeInfo struct {
	Id        int32  `json:"id:`
	Name      string `json:"name"`
	Url       string `json:"url"`
	PublicKey []byte `json:"public_key"`
	IsNormal  bool   `json:"is_normal"`
}

// MultiSigInfo 多签信息
type MultiSigInfo struct {
	BtcAddress      string `json:"btc_address"`
	BtcRedeemScript []byte `json:"btc_redeem_script"`
	BchAddress      string `json:"bch_address"`
	BchRedeemScript []byte `json:"bch_redeem_script"`
}

// Snapshot 集群快照
type Snapshot struct {
	NodeList       []NodeInfo `json:"node_list"`
	ClusterSize    int        `json:"cluster_size"`
	TotalNodeCount int        `json:"total_node_count"`
	QuorumN        int        `json:"quorum_n"`
	AccuseQuorumN  int        `json:"accuse_quorum_n"`
}

// 节点的启动模式
const (
	_ = iota
	// ModeNormal 正常启动模式，参与共识
	ModeNormal
	// ModeJoin 以新节点加入集群的模式启动
	ModeJoin
	// ModeWatch 以观察模式启动，不参与共识，只同步区块
	ModeWatch
	// ModeTest 以观察模式启动，但不同步区块，数据通过接口伪造
	ModeTest
)

// Equal 比较两个MultiSigInfo是否相等
func (ms MultiSigInfo) Equal(other MultiSigInfo) bool {
	return ms.BchAddress == other.BchAddress && bytes.Equal(ms.BchRedeemScript, other.BchRedeemScript) &&
		ms.BtcAddress == other.BtcAddress && bytes.Equal(ms.BtcRedeemScript, other.BtcRedeemScript)
}
