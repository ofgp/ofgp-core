package primitives

import (
	"dgateway/crypto"
	pb "dgateway/proto"
)

const (
	IsTestNet         = true
	FederationAddress = ""
)

var (
	GenesisBlockPack   *pb.BlockPack
	baseBchBlockHeight int
)

func init() {
	// TODO 调用命令行从BCH节点获取当前的高度
	baseBchBlockHeight = 100
	baseBchBlockId := [32]byte{1}
	baseBchPrevId := [32]byte{2}
	baseBchBlockHeader := &pb.BchBlockHeader{
		Height:  int32(baseBchBlockHeight),
		BlockId: &crypto.Digest256{baseBchBlockId[:]},
		PrevId:  &crypto.Digest256{baseBchPrevId[:]},
	}

	genesisBlock := &pb.Block{
		TimestampMs:    0,
		Type:           pb.Block_GENESIS,
		BchBlockHeader: baseBchBlockHeader,
	}
	genesisBlock.UpdateBlockId()

	GenesisBlockPack = pb.NewBlockPack(&pb.InitMsg{
		Term:   0,
		Height: 0,
		Block:  genesisBlock,
	})
}
