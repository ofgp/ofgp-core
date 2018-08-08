package util

import (
	"dgateway/crypto"
	"encoding/hex"
	"strconv"
)

func FeedText(hasher *crypto.Hasher256, data string) {
	hasher.Feed([]byte(data))
}

func FeedBin(hasher *crypto.Hasher256, data []byte) {
	converted := make([]byte, hex.EncodedLen(len(data)))
	hex.Encode(converted, data)
	hasher.Feed(converted)
}

type FeedBodyFn func(*crypto.Hasher256)

func FeedField(hasher *crypto.Hasher256, fieldName string, fn FeedBodyFn) {
	FeedText(hasher, "<"+fieldName+" ")
	fn(hasher)
	FeedText(hasher, ">")
}

func FeedTextField(hasher *crypto.Hasher256, fieldName string, data string) {
	FeedText(hasher, "<"+fieldName+" ")
	FeedText(hasher, data)
	FeedText(hasher, ">")
}

func FeedBinField(hasher *crypto.Hasher256, fieldName string, data []byte) {
	FeedText(hasher, "<"+fieldName+" ")
	FeedBin(hasher, data)
	FeedText(hasher, ">")
}

func FeedInt32Field(hasher *crypto.Hasher256, fieldName string, n int32) {
	FeedTextField(hasher, fieldName, strconv.FormatInt(int64(n), 10))
}

func FeedInt64Field(hasher *crypto.Hasher256, fieldName string, n int64) {
	FeedTextField(hasher, fieldName, strconv.FormatInt(n, 10))
}

func FeedTimestampField(hasher *crypto.Hasher256, fieldName string, ts int64) {
	FeedInt64Field(hasher, fieldName, ts)
}
func FeedDigestField(hasher *crypto.Hasher256, fieldName string, digest *crypto.Digest256) {
	FeedBinField(hasher, fieldName, digest.Data)
}
