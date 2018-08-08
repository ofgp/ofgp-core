package crypto

import (
	"crypto/sha256"
	"fmt"
	"hash"
)

type Hasher256 struct {
	hasher hash.Hash
}

func NewHasher256() *Hasher256 {
	return &Hasher256{sha256.New()}
}

func (hasher *Hasher256) Feed(data []byte) *Hasher256 {
	_, err := hasher.hasher.Write(data)
	if err != nil {
		panic(fmt.Errorf("hash256: Feed encounters unexpected error %v", err))
	}
	return hasher
}

func (hasher *Hasher256) Sum(data []byte) *Digest256 {
	sum := hasher.hasher.Sum(data)
	return &Digest256{sum[:]}
}

func (hasher *Hasher256) Size() int {
	return hasher.hasher.Size()
}

func (hasher *Hasher256) Reset() *Hasher256 {
	hasher.hasher.Reset()
	return hasher
}

func Hash256(data []byte) *Digest256 {
	sum := sha256.Sum256(data)
	return &Digest256{sum[:]}
}
