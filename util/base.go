package util

import (
	"fmt"
	"time"
)

func NowMs() int64 {
	now := time.Now()
	return now.Unix()*1000 + int64(now.Nanosecond())/1e6
}

// MsToTime unix timestampMs to time.Time
func MsToTime(timeMs int64) time.Time {
	second := timeMs / 1000
	nanoSecond := timeMs % 1000 * 1e6
	return time.Unix(second, nanoSecond)
}

func U64ToBytes(v uint64) []byte {
	ret := make([]byte, 8)

	ret[7] = byte(v)
	v >>= 8
	ret[6] = byte(v)
	v >>= 8
	ret[5] = byte(v)
	v >>= 8
	ret[4] = byte(v)
	v >>= 8
	ret[3] = byte(v)
	v >>= 8
	ret[2] = byte(v)
	v >>= 8
	ret[1] = byte(v)
	v >>= 8
	ret[0] = byte(v)

	return ret
}

func I64ToBytes(v int64) []byte {
	return U64ToBytes(uint64(v))
}

func BytesToU64(bytes []byte) (v uint64, e error) {
	if len(bytes) != 8 {
		e = fmt.Errorf("invalid input: %s", bytes)
		return
	}

	v = uint64(bytes[0])
	v <<= 8
	v += uint64(bytes[1])
	v <<= 8
	v += uint64(bytes[2])
	v <<= 8
	v += uint64(bytes[3])
	v <<= 8
	v += uint64(bytes[4])
	v <<= 8
	v += uint64(bytes[5])
	v <<= 8
	v += uint64(bytes[6])
	v <<= 8
	v += uint64(bytes[7])

	return
}

func BytesToI64(bytes []byte) (v int64, e error) {
	u, e := BytesToU64(bytes)
	v = int64(u)
	return
}
