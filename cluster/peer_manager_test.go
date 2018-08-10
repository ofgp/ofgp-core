package cluster_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/node"
	pb "github.com/ofgp/ofgp-core/proto"

	"google.golang.org/grpc"
)

func startServer(port int) *grpc.Server {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(fmt.Sprintf("failed to listen: %v", err))
	}
	grpcServer := grpc.NewServer(grpc.InitialWindowSize(1048576), grpc.InitialConnWindowSize(10485760))
	braftNode := &node.BraftNode{}
	pb.RegisterBraftServer(grpcServer, braftNode)
	go func() {
		grpcServer.Serve(lis)
	}()
	return grpcServer
}

func TestFetch(t *testing.T) {
	server0 := startServer(10001)
	defer server0.Stop()
	nodeInfo0 := cluster.NodeInfo{
		Id:   0,
		Name: "node0",
		Url:  "127.0.0.1:10001",
	}
	server1 := startServer(10002)
	defer server1.Stop()
	nodeInfo1 := cluster.NodeInfo{
		Id:   1,
		Name: "node1",
		Url:  "127.0.0.1:10002",
	}

	cluster.NodeList = []cluster.NodeInfo{nodeInfo0, nodeInfo1}
	pm := cluster.NewPeerManager(0, 5, 2)
	cnt := pm.GetTxConnAvailableCnt(cluster.NodeList)
	t.Logf("get availble cnt:%d", cnt)
	pm.NotifySignTx(0, &pb.SignTxRequest{
		Term: -1,
	})
	pm.NotifyCommitMsg(1, &pb.CommitMsg{
		Term: -1,
	})
}

func TestSnapshot(t *testing.T) {
	nodeInfo0 := cluster.NodeInfo{
		Id:       0,
		Name:     "node0",
		Url:      "127.0.0.1:10001",
		IsNormal: true,
	}
	nodeInfo1 := cluster.NodeInfo{
		Id:       1,
		Name:     "node1",
		Url:      "127.0.0.1:10002",
		IsNormal: true,
	}
	cluster.NodeList = []cluster.NodeInfo{nodeInfo0, nodeInfo1}
	cluster.ClusterSize = 2
	cluster.TotalNodeCount = 2
	cluster.QuorumN = 1
	cluster.AccuseQuorumN = 1
	cluster.CreateSnapShot()
	cluster.DeleteNode(0)
	t.Logf("snapshot:%v,nodeList:%v", cluster.ClusterSnapshot, cluster.NodeList)
	t.Logf("ClusterSize:%d,snapshotSize:%d", cluster.ClusterSize, cluster.ClusterSnapshot.ClusterSize)
}
