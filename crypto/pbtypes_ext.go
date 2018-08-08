package crypto

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

//--------------Digest256

func (d *Digest256) EqualTo(other *Digest256) bool {
	return bytes.Equal(d.Data, other.Data)
}

func (d *Digest256) AsMapKey() string {
	return string(d.Data)
}

func (d *Digest256) ToText() string {
	return hex.EncodeToString(d.Data)
}

func (d Digest256) IsValid() bool {
	return len(d.Data) == 256/8
}

func TextToDigest256(text string) (*Digest256, error) {
	bytes, err := hex.DecodeString(text)
	if err != nil {
		return nil, err
	}
	digest := &Digest256{bytes}
	if !digest.IsValid() {
		err = fmt.Errorf("Invalid digest: %v", text)
		digest = nil
	}
	return digest, err
}

func NewDigest256(bytes []byte) (*Digest256, error) {
	data := make([]byte, len(bytes))
	copy(data, bytes)
	digest := &Digest256{data}
	var err error
	if !digest.IsValid() {
		err = fmt.Errorf("Invalid digest: %v", bytes)
		digest = nil
	}

	return digest, err
}
