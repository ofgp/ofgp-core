package cluster

import (
	"fmt"
	"sync"
	"time"

	"github.com/ofgp/ofgp-core/crypto"
	"github.com/ofgp/ofgp-core/log"
	pb "github.com/ofgp/ofgp-core/proto"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

const (
	rpcTimeout                = 5 * time.Second
	connectionTimeout         = 5 * time.Second
	connectionPoolSizePerPeer = 200
)

var (
	errInvalidNode       = fmt.Errorf("Node is invalid or node not found")
	errConnPoolExhausted = fmt.Errorf("Connection pool exhausted")

	pmLogger = log.New(viper.GetString("loglevel"), "pm")
)

// PeerNode 维持节点的链接
type PeerNode struct {
	NodeInfo
	txConnPool    *connPool
	blockConnPool *connPool
}

// NewPeerNode 新建一个PeerNode对象并返回
func NewPeerNode(nodeInfo NodeInfo, txPoolSize, blockPoolSize int) *PeerNode {
	return &PeerNode{
		NodeInfo:      nodeInfo,
		txConnPool:    newConnPool(nodeInfo.Url, txPoolSize),
		blockConnPool: newConnPool(nodeInfo.Url, blockPoolSize),
	}
}

func newConnPool(url string, size int) *connPool {
	return &connPool{
		url:   url,
		conns: make([]*grpc.ClientConn, size),
	}
}

type connPool struct {
	sync.Mutex
	url   string
	conns []*grpc.ClientConn
}

// isCoonExhausted 连接池是否耗尽
func (pool *connPool) isCoonExhausted() bool {
	pool.Lock()
	defer pool.Unlock()
	return len(pool.conns) == 0
}

// fetchConnectionFromPool 从连接池获取连接
func (pool *connPool) fetchConnectionFromPool() (conn *grpc.ClientConn, err error) {
	pool.Lock()
	defer pool.Unlock()

	poolSize := len(pool.conns)
	if poolSize == 0 {
		err = errConnPoolExhausted
	} else {
		conn = pool.conns[poolSize-1]
		pool.conns = pool.conns[:poolSize-1]

		if conn != nil {
			state := conn.GetState()
			// TransientFailure need re-Dial ??
			if state == connectivity.Shutdown || state == connectivity.TransientFailure {
				conn.Close()
				conn = nil
			}
		}

		if conn == nil {
			conn, err = grpc.Dial(pool.url, grpc.WithInsecure(), grpc.WithTimeout(connectionTimeout),
				grpc.FailOnNonTempDialError(true))
			if err != nil {
				if conn != nil {
					conn.Close()
					conn = nil
				}
				// 创建连接失败 维持连接池长度
				pool.conns = append(pool.conns, nil)
			}
		}
	}

	return

}

// putConnectionBackToPool 将连接放回池子
func (pool *connPool) putConnectionBackToPool(conn *grpc.ClientConn) {
	pool.Lock()
	defer pool.Unlock()
	pool.conns = append(pool.conns, conn)
}

var txMethods = [2]string{"/proto.Braft/NotifySignTx", "/proto.Braft/NotifySignedResult"}

//根据方法名称获取连接池
func (peer *PeerNode) fetchConnPoolByMethod(method string) *connPool {
	for _, m := range txMethods {
		if m == method {
			return peer.txConnPool
		}
	}
	return peer.blockConnPool
}

//  isPoolExhausted 连接池是否耗尽
func (peer *PeerNode) isPoolExhausted() (txExhausted, blockExhausted bool) {
	return peer.txConnPool.isCoonExhausted(), peer.blockConnPool.isCoonExhausted()
}

func (peer *PeerNode) invokeRPC(method string, req, rsp interface{}) error {
	pmLogger.Debug("begin invoke rpc", "method", method, "nodeId", peer.Id)
	connPool := peer.fetchConnPoolByMethod(method)
	conn, err := connPool.fetchConnectionFromPool()
	if err != nil {
		pmLogger.Warn(err.Error())
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	defer connPool.putConnectionBackToPool(conn)

	err = conn.Invoke(ctx, method, req, rsp, grpc.FailFast(true))
	if err != nil {
		pmLogger.Debug(err.Error(), "method", method)
	}
	return err
}

// PeerManager 管理所有PeerNode的结构体, 负责广播消息
type PeerManager struct {
	localNodeId       int32
	txConnPoolSize    int
	blockCoonPoolSize int
	peers             map[int32]*PeerNode
	sync.RWMutex
}

// NewPeerManager 新建一个PeerManager对象并返回
func NewPeerManager(localNodeId int32, txConnPoolSize, blockCoonPoolSize int) *PeerManager {
	peers := make(map[int32]*PeerNode, ClusterSize)
	for _, nodeInfo := range NodeList {
		if nodeInfo.IsNormal {
			peers[nodeInfo.Id] = NewPeerNode(nodeInfo, txConnPoolSize, blockCoonPoolSize)
		}
	}

	return &PeerManager{
		localNodeId:       localNodeId,
		txConnPoolSize:    txConnPoolSize,
		blockCoonPoolSize: blockCoonPoolSize,
		peers:             peers,
	}
}

// AddNode 增加节点到manager
func (pm *PeerManager) AddNode(nodeInfo NodeInfo) {
	peerNode := NewPeerNode(nodeInfo, pm.txConnPoolSize, pm.blockCoonPoolSize)
	pm.Lock()
	pm.peers[nodeInfo.Id] = peerNode
	pm.Unlock()
}

// DeleteNode del node
func (pm *PeerManager) DeleteNode(id int32) {
	pm.Lock()
	delete(pm.peers, id)
	pm.Unlock()
}

// LocalNodeId 返回本节点的节点ID
func (pm *PeerManager) LocalNodeId() int32 {
	return pm.localNodeId
}

// GetNode 根据指定的节点ID返回对应的PeerNode
func (pm *PeerManager) GetNode(nodeId int32) *PeerNode {
	return pm.peers[nodeId]
}

// Broadcast 把msg广播给其他节点
// excludeSelf 是否排除本节点，true不发给本节点，false也发给本节点
// selectSome 是否仅发送部分节点
func (pm *PeerManager) Broadcast(msg interface{}, excludeSelf bool, selectSome bool) {
	sentN := 0
	selectSomeN := AccuseQuorumN + 2

	pm.RLock()
	for nodeId := range pm.peers {
		if !NodeList[nodeId].IsNormal {
			continue
		}
		if excludeSelf && nodeId == pm.localNodeId {
			continue
		}
		if selectSome && sentN >= selectSomeN {
			break
		}
		nodeId := nodeId
		sentN++
		switch msg := msg.(type) {
		case *pb.InitMsg:
			pmLogger.Debug("Broadcast init msg", "nodeId", nodeId)
			go func() { pm.NotifyInitMsg(nodeId, msg) }()
		case *pb.PrepareMsg:
			pmLogger.Debug("Broadcast prepare msg", "nodeId", nodeId)
			go func() { pm.NotifyPrepareMsg(nodeId, msg) }()
		case *pb.CommitMsg:
			pmLogger.Debug("Broadcast commit msg", "nodeId", nodeId)
			go func() { pm.NotifyCommitMsg(nodeId, msg) }()
		case *pb.WeakAccuse:
			pmLogger.Debug("Broadcast weakaccuse msg", "nodeId", nodeId)
			go func() { pm.NotifyWeakAccuse(nodeId, msg) }()
		case *pb.StrongAccuse:
			pmLogger.Debug("Broadcast strongaccuse msg", "nodeId", nodeId)
			go func() { pm.NotifyStrongAccuse(nodeId, msg) }()
		case *pb.Vote:
			pmLogger.Debug("Broadcast vote msg", "nodeId", nodeId)
			go func() { pm.NotifyVote(nodeId, msg) }()
		case *pb.Transactions:
			pmLogger.Debug("Broadcast transactions msg", "nodeId", nodeId)
			go func() { pm.NotifyTxs(nodeId, msg) }()
		case *pb.SignTxRequest:
			pmLogger.Debug("Broadcast sign tx request", "nodeId", nodeId)
			go func() { pm.NotifySignTx(nodeId, msg) }()
		case *pb.SignedResult:
			pmLogger.Debug("Broadcast signed result", "nodeId", nodeId)
			go func() { pm.NotifySignedResult(nodeId, msg) }()
		case *pb.LeaveRequest:
			// leave是最后发送的消息，需要确认发送完成才能退出
			pmLogger.Debug("Broadcast leave request", "nodeId", nodeId)
			pm.NotifyLeave(nodeId, msg)
		default:
			pmLogger.Error("Invalid notification type.")
		}
	}
	pm.RUnlock()
}

// SyncUp 向nodeId节点发送Syncup消息
func (pm *PeerManager) SyncUp(nodeId int32, req *pb.SyncUpRequest) (*pb.SyncUpResponse, error) {
	node := pm.GetNode(nodeId)
	if node == nil || nodeId == pm.localNodeId {
		return nil, errInvalidNode
	}

	rsp := new(pb.SyncUpResponse)
	err := node.invokeRPC("/proto.Braft/SyncUp", req, rsp)
	return rsp, err
}

func (pm *PeerManager) WatchSyncUp(nodeId int32, req *pb.SyncUpRequest) (*pb.SyncUpResponse, error) {
	node := pm.GetNode(nodeId)
	if node == nil || nodeId == pm.localNodeId {
		return nil, errInvalidNode
	}

	rsp := new(pb.SyncUpResponse)
	err := node.invokeRPC("/proto.Braft/WatchSyncUp", req, rsp)
	return rsp, err
}

// NotifyInitMsg 向nodeid节点发送InitMsg消息
func (pm *PeerManager) NotifyInitMsg(nodeId int32, msg *pb.InitMsg) (*pb.Void, error) {
	node := pm.GetNode(nodeId)
	if node == nil {
		return nil, errInvalidNode
	}
	rsp := new(pb.Void)
	err := node.invokeRPC("/proto.Braft/NotifyInitMsg", msg, rsp)
	return rsp, err
}

// NotifyPrepareMsg 向nodeid节点发送PrepareMsg消息
func (pm *PeerManager) NotifyPrepareMsg(nodeId int32, msg *pb.PrepareMsg) (*pb.Void, error) {
	node := pm.GetNode(nodeId)
	if node == nil {
		return nil, errInvalidNode
	}
	rsp := new(pb.Void)
	err := node.invokeRPC("/proto.Braft/NotifyPrepareMsg", msg, rsp)
	return rsp, err
}

// NotifyCommitMsg 向nodeid节点发送CommitMsg消息
func (pm *PeerManager) NotifyCommitMsg(nodeId int32, msg *pb.CommitMsg) (*pb.Void, error) {
	node := pm.GetNode(nodeId)
	if node == nil {
		return nil, errInvalidNode
	}
	rsp := new(pb.Void)
	err := node.invokeRPC("/proto.Braft/NotifyCommitMsg", msg, rsp)
	return rsp, err
}

// NotifyWeakAccuse 向nodeid节点发送WeakAccuse消息
func (pm *PeerManager) NotifyWeakAccuse(nodeId int32, msg *pb.WeakAccuse) (*pb.Void, error) {
	node := pm.GetNode(nodeId)
	if node == nil {
		return nil, errInvalidNode
	}
	rsp := new(pb.Void)
	err := node.invokeRPC("/proto.Braft/NotifyWeakAccuse", msg, rsp)
	return rsp, err
}

// NotifyStrongAccuse 向nodeid节点发送StrongAccuse消息
func (pm *PeerManager) NotifyStrongAccuse(nodeId int32, msg *pb.StrongAccuse) (*pb.Void, error) {
	node := pm.GetNode(nodeId)
	if node == nil {
		return nil, errInvalidNode
	}
	rsp := new(pb.Void)
	err := node.invokeRPC("/proto.Braft/NotifyStrongAccuse", msg, rsp)
	return rsp, err
}

// NotifyVote 向nodeid节点发送Vote消息
func (pm *PeerManager) NotifyVote(nodeId int32, msg *pb.Vote) (*pb.Void, error) {
	node := pm.GetNode(nodeId)
	if node == nil {
		return nil, errInvalidNode
	}
	rsp := new(pb.Void)
	err := node.invokeRPC("/proto.Braft/NotifyVote", msg, rsp)
	return rsp, err
}

// NotifyTxs 向nodeid节点发送Transaction, 已废弃
func (pm *PeerManager) NotifyTxs(nodeId int32, msg *pb.Transactions) (*pb.Void, error) {
	node := pm.GetNode(nodeId)
	if node == nil {
		return nil, errInvalidNode
	}
	rsp := new(pb.Void)
	err := node.invokeRPC("/proto.Braft/NotifyTxs", msg, rsp)
	return rsp, err
}

// NotifySignTx 向nodeid节点发送SignTxRequest消息
func (pm *PeerManager) NotifySignTx(nodeId int32, msg *pb.SignTxRequest) (*pb.SignTxResponse, error) {
	node := pm.GetNode(nodeId)
	if node == nil {
		return nil, errInvalidNode
	}
	rsp := new(pb.SignTxResponse)
	err := node.invokeRPC("/proto.Braft/NotifySignTx", msg, rsp)
	return rsp, err
}

// NotifySignedResult 向nodeid节点发送SignedResult消息
func (pm *PeerManager) NotifySignedResult(nodeId int32, msg *pb.SignedResult) (*pb.Void, error) {
	node := pm.GetNode(nodeId)
	if node == nil {
		return nil, errInvalidNode
	}
	rsp := new(pb.Void)
	err := node.invokeRPC("/proto.Braft/NotifySignedResult", msg, rsp)
	return rsp, err
}

// NotifyLeave 向nodeid节点发送LeaveRequest消息
func (pm *PeerManager) NotifyLeave(nodeId int32, msg *pb.LeaveRequest) (*pb.Void, error) {
	node := pm.GetNode(nodeId)
	if node == nil {
		return nil, errInvalidNode
	}
	rsp := new(pb.Void)
	err := node.invokeRPC("/proto.Braft/NotifyLeave", msg, rsp)
	return rsp, err
}

func (pm *PeerManager) GetCommitedCnt(msg *crypto.Digest256) int {
	var cnt int
	peerSize := len(pm.peers)
	ch := make(chan *pb.IsCommitedResponse, peerSize)
	var wg sync.WaitGroup
	wg.Add(peerSize)
	for _, peerNode := range pm.peers {
		peerNodeNow := peerNode
		go func() {
			defer wg.Done()
			res := new(pb.IsCommitedResponse)
			err := peerNodeNow.invokeRPC("/proto.Braft/IsCommited", msg, res)
			if err != nil {
				return
			}
			ch <- res
		}()
	}
	wg.Wait()
	close(ch)
	for res := range ch {
		if res != nil && res.Commited {
			cnt++
		}
	}
	return cnt
}

//获取节点运行时的info,监听的区块的高度和成为leader的次数
func (pm *PeerManager) GetNodeRuntimeInfos() map[int32]*pb.NodeRuntimeInfo {
	size := len(pm.peers)
	if size == 0 {
		return nil
	}
	ch := make(chan *pb.NodeRuntimeInfo, size)
	var wg sync.WaitGroup
	wg.Add(size)
	pm.RLock()
	for _, node := range pm.peers {
		nodeNow := node
		go func() {
			defer wg.Done()
			apiData := &pb.NodeRuntimeInfo{}
			err := nodeNow.invokeRPC("/proto.Braft/GetNodeRuntimeInfo", &pb.Void{}, apiData)
			if err != nil {
				pmLogger.Error(err.Error())
				return
			}
			ch <- apiData
		}()
	}
	pm.RUnlock()
	wg.Wait()
	close(ch)
	apiDatas := make(map[int32]*pb.NodeRuntimeInfo)
	for data := range ch {
		if data != nil {
			apiDatas[data.NodeID] = data
		}
	}
	return apiDatas
}

func (pm *PeerManager) GetTxConnAvailableCnt(nodes []NodeInfo) int {
	pm.RLock()
	defer pm.RUnlock()
	var cnt int
	for _, node := range nodes {
		peer := pm.peers[node.Id]
		if available, _ := peer.isPoolExhausted(); !available {
			cnt++
		}
	}
	return cnt
}
