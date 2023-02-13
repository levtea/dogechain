package memorydb

import (
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/dogechain-lab/dogechain/types"
)

var (
	// errMemorydbClosed is returned if a memory database was already closed at the
	// invocation of a data access operation.
	errMemorydbClosed = errors.New("database closed")

	// errMemorydbNotFound is returned if a key is requested that is not found in
	// the provided memory database.
	errMemorydbNotFound = errors.New("not found")

	// errSnapshotReleased is returned if callers want to retrieve data from a
	// released snapshot.
	errSnapshotReleased = errors.New("snapshot released")
)

// Database is an ephemeral key-value store. Apart from basic data storage
// functionality it also supports batch writes and iterating over the keyspace in
// binary-alphabetical order.
type Database struct {
	db   map[string][]byte
	lock sync.RWMutex
}

// New returns a wrapped map with all the required database interface methods
// implemented.
func New() *Database {
	return &Database{
		db: make(map[string][]byte),
	}
}

// NewWithCap returns a wrapped map pre-allocated to the provided capacity with
// all the required database interface methods implemented.
func NewWithCap(size int) *Database {
	return &Database{
		db: make(map[string][]byte, size),
	}
}

// Close deallocates the internal map and ensures any consecutive data access op
// fails with an error.
func (db *Database) Close() error {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.db = nil

	return nil
}

// Has retrieves if a key is present in the key-value store.
func (db *Database) Has(key []byte) (bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	if db.db == nil {
		return false, errMemorydbClosed
	}

	_, ok := db.db[string(key)]

	return ok, nil
}

// Get retrieves the given key if it's present in the key-value store.
func (db *Database) Get(key []byte) ([]byte, bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	if db.db == nil {
		return nil, false, errMemorydbClosed
	}

	if entry, ok := db.db[string(key)]; ok {
		return types.CopyBytes(entry), true, nil
	}

	return nil, false, errMemorydbNotFound
}

// Set inserts the given value into the key-value store.
func (db *Database) Set(key []byte, value []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	if db.db == nil {
		return errMemorydbClosed
	}

	db.db[string(key)] = types.CopyBytes(value)

	return nil
}

// Delete removes the key from the key-value store.
func (db *Database) Delete(key []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	if db.db == nil {
		return errMemorydbClosed
	}

	delete(db.db, string(key))

	return nil
}

// NewBatch creates a write-only key-value store that buffers changes to its host
// database until a final write is called.
func (db *Database) NewBatch() kvdb.Batch {
	return &batch{
		db: db,
	}
}

// NewIterator creates a binary-alphabetical iterator over a subset
// of database content with a particular key prefix, starting at a particular
// initial key (or after, if it does not exist).
func (db *Database) NewIterator(prefix []byte, start []byte) kvdb.Iterator {
	db.lock.RLock()
	defer db.lock.RUnlock()

	var (
		pr     = string(prefix)
		st     = string(append(prefix, start...))
		keys   = make([]string, 0, len(db.db))
		values = make([][]byte, 0, len(db.db))
	)

	// Collect the keys from the memory database corresponding to the given prefix
	// and start
	for key := range db.db {
		if !strings.HasPrefix(key, pr) {
			continue
		}

		if key >= st {
			keys = append(keys, key)
		}
	}

	// Sort the items and retrieve the associated values
	sort.Strings(keys)

	for _, key := range keys {
		values = append(values, db.db[key])
	}

	return &iterator{
		index:  -1,
		keys:   keys,
		values: values,
	}
}

// NewSnapshot creates a database snapshot based on the current state.
// The created snapshot will not be affected by all following mutations
// happened on the database.
func (db *Database) NewSnapshot() (kvdb.Snapshot, error) {
	return newSnapshot(db), nil
}

// Stat returns a particular internal stat of the database.
func (db *Database) Stat(property string) (string, error) {
	return "", errors.New("unknown property")
}

// Compact is not supported on a memory database, but there's no need either as
// a memory database doesn't waste space anyway.
func (db *Database) Compact(start []byte, limit []byte) error {
	return nil
}

// Len returns the number of entries currently present in the memory database.
//
// Note, this method is only used for testing (i.e. not public in general) and
// does not have explicit checks for closed-ness to allow simpler testing code.
func (db *Database) Len() int {
	db.lock.RLock()
	defer db.lock.RUnlock()

	return len(db.db)
}

// keyvalue is a key-value tuple tagged with a deletion field to allow creating
// memory-database write batches.
type keyvalue struct {
	key    []byte
	value  []byte
	delete bool
}

// batch is a write-only memory batch that commits changes to its host
// database when Write is called. A batch cannot be used concurrently.
type batch struct {
	db     *Database
	writes []keyvalue
	size   int
}

// Set inserts the given value into the batch for later committing.
func (b *batch) Set(key, value []byte) error {
	b.writes = append(b.writes, keyvalue{types.CopyBytes(key), types.CopyBytes(value), false})
	b.size += len(key) + len(value)

	return nil
}

// Delete inserts the a key removal into the batch for later committing.
func (b *batch) Delete(key []byte) error {
	b.writes = append(b.writes, keyvalue{types.CopyBytes(key), nil, true})
	b.size += len(key)

	return nil
}

// ValueSize retrieves the amount of data queued up for writing.
func (b *batch) ValueSize() int {
	return b.size
}

// Write flushes any accumulated data to the memory database.
func (b *batch) Write() error {
	b.db.lock.Lock()
	defer b.db.lock.Unlock()

	for _, keyvalue := range b.writes {
		if keyvalue.delete {
			delete(b.db.db, string(keyvalue.key))

			continue
		}

		b.db.db[string(keyvalue.key)] = keyvalue.value
	}

	return nil
}

// Reset resets the batch for reuse.
func (b *batch) Reset() {
	b.writes = b.writes[:0]
	b.size = 0
}

// Replay replays the batch contents.
func (b *batch) Replay(w kvdb.KVWriter) error {
	for _, keyvalue := range b.writes {
		if keyvalue.delete {
			if err := w.Delete(keyvalue.key); err != nil {
				return err
			}

			continue
		}

		if err := w.Set(keyvalue.key, keyvalue.value); err != nil {
			return err
		}
	}

	return nil
}

// iterator can walk over the (potentially partial) keyspace of a memory key
// value store. Internally it is a deep copy of the entire iterated state,
// sorted by keys.
type iterator struct {
	index  int
	keys   []string
	values [][]byte
}

// Next moves the iterator to the next key/value pair. It returns whether the
// iterator is exhausted.
func (it *iterator) Next() bool {
	// Short circuit if iterator is already exhausted in the forward direction.
	if it.index >= len(it.keys) {
		return false
	}

	it.index += 1

	return it.index < len(it.keys)
}

// Error returns any accumulated error. Exhausting all the key/value pairs
// is not considered to be an error. A memory iterator cannot encounter errors.
func (it *iterator) Error() error {
	return nil
}

// Key returns the key of the current key/value pair, or nil if done. The caller
// should not modify the contents of the returned slice, and its contents may
// change on the next call to Next.
func (it *iterator) Key() []byte {
	// Short circuit if iterator is not in a valid position
	if it.index < 0 || it.index >= len(it.keys) {
		return nil
	}

	return []byte(it.keys[it.index])
}

// Value returns the value of the current key/value pair, or nil if done. The
// caller should not modify the contents of the returned slice, and its contents
// may change on the next call to Next.
func (it *iterator) Value() []byte {
	// Short circuit if iterator is not in a valid position
	if it.index < 0 || it.index >= len(it.keys) {
		return nil
	}

	return it.values[it.index]
}

// Release releases associated resources. Release should always succeed and can
// be called multiple times without causing error.
func (it *iterator) Release() {
	it.index, it.keys, it.values = -1, nil, nil
}

// snapshot wraps a batch of key-value entries deep copied from the in-memory
// database for implementing the Snapshot interface.
type snapshot struct {
	db   map[string][]byte
	lock sync.RWMutex
}

// newSnapshot initializes the snapshot with the given database instance.
func newSnapshot(db *Database) *snapshot {
	db.lock.RLock()
	defer db.lock.RUnlock()

	copied := make(map[string][]byte)
	for key, val := range db.db {
		copied[key] = types.CopyBytes(val)
	}

	return &snapshot{db: copied}
}

// Has retrieves if a key is present in the snapshot backing by a key-value
// data store.
func (snap *snapshot) Has(key []byte) (bool, error) {
	snap.lock.RLock()
	defer snap.lock.RUnlock()

	if snap.db == nil {
		return false, errSnapshotReleased
	}

	_, ok := snap.db[string(key)]

	return ok, nil
}

// Get retrieves the given key if it's present in the snapshot backing by
// key-value data store.
func (snap *snapshot) Get(key []byte) ([]byte, error) {
	snap.lock.RLock()
	defer snap.lock.RUnlock()

	if snap.db == nil {
		return nil, errSnapshotReleased
	}

	if entry, ok := snap.db[string(key)]; ok {
		return types.CopyBytes(entry), nil
	}

	return nil, errMemorydbNotFound
}

// Release releases associated resources. Release should always succeed and can
// be called multiple times without causing error.
func (snap *snapshot) Release() {
	snap.lock.Lock()
	defer snap.lock.Unlock()

	snap.db = nil
}