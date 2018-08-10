package dgwdb_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/ofgp/ofgp-core/util"
)

var testValues = []string{"aaa", "bbb", "ccc"}

func newTestLDB() (*dgwdb.LDBDatabase, func()) {
	dirname, err := ioutil.TempDir(os.TempDir(), "braft_test_")
	if err != nil {
		panic("failed to create test file: " + err.Error())
	}
	db, err := dgwdb.NewLDBDatabase(dirname, 0, 0)
	if err != nil {
		panic("failed to create test database: " + err.Error())
	}

	return db, func() {
		db.Close()
		os.RemoveAll(dirname)
	}
}

func TestLDB_PutGet(t *testing.T) {
	db, remove := newTestLDB()
	defer remove()
	testPutGet(db, t)
}
func TestRange(t *testing.T) {
	db, _ := dgwdb.NewLDBDatabase("/Users/bitmain/leveldb_data", 0, 0)
	defer db.Close()
	prefix := []byte("B")
	keyStart := append(prefix, util.I64ToBytes(1)...)
	keyEnd := append(prefix, util.I64ToBytes(3)...)
	iter := db.NewIteraterWithRange(keyStart, keyEnd)

	for iter.Next() {
		t.Logf("k:%v,v:%v", iter.Key(), iter.Value())
	}
}
func TestLDBRange(t *testing.T) {
	db, remove := newTestLDB()
	defer remove()
	prefix := []byte("test")
	var i int64
	for i = 0; i < 5; i++ {
		value := util.I64ToBytes(i)
		keySUf := util.I64ToBytes(i)
		key := append(prefix, keySUf...)
		db.Put(key, value)
	}
	keyStart := append(prefix, util.I64ToBytes(0)...)
	keyEnd := append(prefix, util.I64ToBytes(5)...)
	iter := db.NewIteraterWithRange(keyStart, keyEnd)

	for iter.Next() {
		t.Logf("k:%v,v:%v", iter.Key(), iter.Value())
	}
}

func testPutGet(db *dgwdb.LDBDatabase, t *testing.T) {
	//t.Parallel()

	t.Log("begin test putget")
	for idx, v := range testValues {
		k := "prefix" + strconv.Itoa(idx)
		//k := util.I64ToBytes(int64(idx))
		err := db.Put([]byte(k), []byte(v))
		if err != nil {
			t.Fatalf("put failed: %v", err)
		}
	}

	for idx, v := range testValues {
		k := "prefix" + strconv.Itoa(idx)
		//k := util.I64ToBytes(int64(idx))
		data, err := db.Get([]byte(k))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if !bytes.Equal(data, []byte(v)) {
			t.Fatalf("get returned wrong result")
		}
	}

	iter := db.NewIterator()
	for idx, v := range testValues {
		k := "prefix" + strconv.Itoa(idx)
		found := iter.Seek([]byte(k))
		if !found {
			t.Fatalf("iterator seek failed: %q", k)
		} else {
			value := iter.Value()
			if !bytes.Equal(value, []byte(v)) {
				t.Fatalf("iterator seek wrong result")
			}
		}
	}
	iter = db.NewIteratorWithPrefix([]byte("prefix"))
	iter.Last()
	value := iter.Value()
	t.Logf("last value is %q", value)
	if !bytes.Equal(value, []byte("ccc")) {
		t.Fatalf("iterator last seek wrong result")
	}
}
