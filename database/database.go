package database

import (
	"encoding/json"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/drep-project/drep-chain/crypto"
	"github.com/drep-project/drep-chain/crypto/sha3"
	chainTypes "github.com/drep-project/drep-chain/chain/types"
    "math/big"
)

type Database struct {
	db     *leveldb.DB
	temp   map[string][]byte
	states map[string]*State
	stores map[string]*chainTypes.Storage
	//trie  Trie
	root   []byte
}

func NewDatabase(dbPath string) (*Database, error){
	ldb, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, err
	}
	db := &Database{
		db:     ldb,
		temp:   nil,
		states: nil,
	}
	db.initState()
	return db,nil
}

func (db *Database) initState() {
	db.root = sha3.Hash256([]byte("state rootState"))
	rootState := &State{
		Sequence: "",
		Value:    []byte{0},
		IsLeaf:   true,
	}
	value, _ := json.Marshal(rootState)
	db.put(db.root, value, false)
}

func (db *Database) get(key []byte, transactional bool) ([]byte, error) {
	if !transactional {
		return db.db.Get(key, nil)
	}
	hk := bytes2Hex(key)
	value, ok := db.temp[hk]
	if !ok {
		var err error
		value, err = db.db.Get(key, nil)
		if err != nil {
			return nil, err
		}
		db.temp[hk] = value
	}
	return value, nil
}

func (db *Database) put(key []byte, value []byte, temporary bool) error {
	if !temporary {
		return db.db.Put(key, value, nil)
	}
	db.temp[bytes2Hex(key)] = value
	return nil
}

func (db *Database) delete(key []byte, temporary bool) error {
	if !temporary {
		return db.db.Delete(key, nil)
	}
	db.temp[bytes2Hex(key)] = nil
	return nil
}

func (db *Database) BeginTransaction() {
	db.temp = make(map[string][]byte)
	db.states = make(map[string]*State)
	db.stores = make(map[string]*chainTypes.Storage)
}

func (db *Database) EndTransaction() {
	db.temp = nil
	db.states = nil
	db.stores = nil
}

func (db *Database) Commit() {
	for key, value := range db.temp {
		bk := hex2Bytes(key)
		if value != nil {
			db.put(bk, value, false)
		} else {
			db.delete(bk, false)
		}
	}
	db.EndTransaction()
}

func (db *Database) Discard() {
	db.EndTransaction()
}

//trie tree root
func (db *Database) getStateRoot() []byte {
	state, _ := db.getState(db.root)
	return state.Value
}

func (db *Database) getState(key []byte) (*State, error) {
	var state *State
	hk := bytes2Hex(key)
	state, ok := db.states[hk]
	if ok {
		return state, nil
	}
	b, err := db.get(key, true)
	if err != nil {
		return nil, err
	}
	state = &State{}
	err = json.Unmarshal(b, state)
	if err != nil {
		return nil, err
	}
	state.db = db
	db.states[hk] = state
	return state, nil
}

func (db *Database) putState(key []byte, state *State) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	err = db.put(key, b, true)
	if err != nil {
		return err
	}
	state.db = db
	db.states[bytes2Hex(key)] = state
	return err
}

func (db *Database) delState(key []byte) error {
	err := db.delete(key, true)
	if err != nil {
		return err
	}
	db.states[bytes2Hex(key)] = nil
	return nil
}

func (db *Database) getStorage(addr *crypto.CommonAddress) *chainTypes.Storage {
	storage := &chainTypes.Storage{}
	key := sha3.Hash256([]byte("storage_" + addr.Hex()))
	value, err := db.get(key, false)
	if err != nil {
	    storage.Balance = new(big.Int)
	    storage.Nonce = 0
	    storage.Reputation = new(big.Int)
		return storage
	}
	json.Unmarshal(value, storage)
	return storage
}

func (db *Database) putStorage(addr *crypto.CommonAddress, storage *chainTypes.Storage) error {
	key := sha3.Hash256([]byte("storage_" + addr.Hex()))
	value, err := json.Marshal(storage)
	if err != nil {
		return err
	}

	return db.put(key, value, true)
}
