package primitives

import (
	pb "github.com/ofgp/ofgp-core/proto"
)

var (
	// GenesisBlockPack 创世区块
	GenesisBlockPack *pb.BlockPack
)

func init() {
	genesisBlock := &pb.Block{
		TimestampMs: 1534766888,
		Type:        pb.Block_GENESIS,
	}
	genesisBlock.UpdateBlockId()

	GenesisBlockPack = pb.NewBlockPack(&pb.InitMsg{
		Term:   0,
		Height: 0,
		Block:  genesisBlock,
	})
}
