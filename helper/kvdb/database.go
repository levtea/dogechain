package kvdb

import "io"

// KVReader wraps the Get method of a backing data store.
type KVReader interface {
	// // Has retrieves if a key is present in the key-value data store.
	// Has(key []byte) (bool, error)
	// Get retrieves the given key if it's present in the key-value data store.
	Get(key []byte) (value []byte, exists bool, err error)
}

// KVWriter wraps the Put method of a backing data store.
type KVWriter interface {
	// Set inserts the given value into the key-value data store.
	Set(k, v []byte) error
	// // Delete removes the key from the key-value data store.
	// Delete(key []byte) error
}

// KVBatchStorage is a batch write for leveldb
type KVBatchStorage interface {
	KVReader
	KVWriter
	Batcher
	Iteratee
	io.Closer
}