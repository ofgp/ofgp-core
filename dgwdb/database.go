package dgwdb

import (
	"sync"

	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"

	log15 "github.com/inconshreveable/log15"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var OpenFileLimit = 64

type LDBDatabase struct {
	filename string
	db       *leveldb.DB
	log      log15.Logger
	quitLock sync.Mutex
	quitChan chan chan error
}

func NewLDBDatabase(file string, cache int, handles int) (*LDBDatabase, error) {
	//logger := log.New()

	if cache < 16 {
		cache = 16
	}
	if handles < 16 {
		handles = 16
	}

	db, err := leveldb.OpenFile(file, &opt.Options{
		OpenFilesCacheCapacity: handles,
		BlockCacheCapacity:     cache / 2 * opt.MiB,
		WriteBuffer:            cache / 4 * opt.MiB,
		Filter:                 filter.NewBloomFilter(10),
	})
	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(file, nil)
	}

	if err != nil {
		return nil, err
	}

	return &LDBDatabase{
		filename: file,
		db:       db,
	}, nil
}

func (db *LDBDatabase) Path() string {
	return db.filename
}

func (db *LDBDatabase) Put(key []byte, value []byte) error {
	return db.db.Put(key, value, nil)
}

func (db *LDBDatabase) Has(key []byte) (bool, error) {
	return db.db.Has(key, nil)
}

func (db *LDBDatabase) Get(key []byte) ([]byte, error) {
	data, err := db.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (db *LDBDatabase) Delete(key []byte) error {
	return db.db.Delete(key, nil)
}

func (db *LDBDatabase) NewIterator() iterator.Iterator {
	return db.db.NewIterator(nil, nil)
}

func (db *LDBDatabase) NewIteratorWithPrefix(prefix []byte) iterator.Iterator {
	return db.db.NewIterator(util.BytesPrefix(prefix), nil)
}

func (db *LDBDatabase) DeleteWithPrefix(prefix []byte) {
	iter := db.NewIteratorWithPrefix(prefix)
	for iter.Next() {
		key := iter.Key()
		db.Delete(key)
	}
}

func (db *LDBDatabase) Close() {
	err := db.db.Close()
	if err == nil {
		//db.log.Info("Database closed")
	} else {
		//db.log.Error("Close database failed", "err", err)
	}
}

func (db *LDBDatabase) LDB() *leveldb.DB {
	return db.db
}

func (db *LDBDatabase) NewIteraterWithRange(start, limit []byte) iterator.Iterator {
	iter := db.db.NewIterator(&util.Range{Start: start, Limit: limit}, nil)
	return iter
}
