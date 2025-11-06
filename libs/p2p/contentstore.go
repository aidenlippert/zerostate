package p2p

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// ContentStore interface for storing and retrieving content
type ContentStore interface {
	Put(ctx context.Context, cid string, data []byte) error
	Get(ctx context.Context, cid string) ([]byte, error)
	Has(ctx context.Context, cid string) (bool, error)
	Delete(ctx context.Context, cid string) error
	Close() error
}

// MemoryContentStore is an in-memory implementation (for testing/dev)
type MemoryContentStore struct {
	mu    sync.RWMutex
	store map[string][]byte
}

// NewMemoryContentStore creates a new in-memory content store
func NewMemoryContentStore() *MemoryContentStore {
	return &MemoryContentStore{
		store: make(map[string][]byte),
	}
}

func (m *MemoryContentStore) Put(ctx context.Context, cid string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[cid] = data
	return nil
}

func (m *MemoryContentStore) Get(ctx context.Context, cid string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, exists := m.store[cid]
	if !exists {
		return nil, fmt.Errorf("content not found")
	}
	return data, nil
}

func (m *MemoryContentStore) Has(ctx context.Context, cid string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.store[cid]
	return exists, nil
}

func (m *MemoryContentStore) Delete(ctx context.Context, cid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, cid)
	return nil
}

func (m *MemoryContentStore) Close() error {
	return nil
}

// Global store instance (will be replaced with node-scoped store)
var (
	globalStore   ContentStore
	globalStoreMu sync.RWMutex
)

func init() {
	globalStore = NewMemoryContentStore()
}

// SetGlobalContentStore sets the global content store instance
func SetGlobalContentStore(store ContentStore) {
	globalStoreMu.Lock()
	defer globalStoreMu.Unlock()
	if globalStore != nil {
		globalStore.Close()
	}
	globalStore = store
}

// GetGlobalContentStore returns the global content store
func GetGlobalContentStore() ContentStore {
	globalStoreMu.RLock()
	defer globalStoreMu.RUnlock()
	return globalStore
}

// Helper functions for backward compatibility
func putContent(ctx context.Context, cid string, data []byte) error {
	return GetGlobalContentStore().Put(ctx, cid, data)
}

func getContent(ctx context.Context, cid string) ([]byte, error) {
	return GetGlobalContentStore().Get(ctx, cid)
}

func hasContent(ctx context.Context, cid string) (bool, error) {
	return GetGlobalContentStore().Has(ctx, cid)
}

// FileContentStore would be implemented here with BadgerDB or LevelDB
// For now, we use the memory store but with proper interface
type FileContentStore struct {
	path   string
	logger *zap.Logger
	// db     *badger.DB // Future: actual persistent storage
	mem *MemoryContentStore // Temporary: use memory for now
}

// NewFileContentStore creates a file-based content store
func NewFileContentStore(path string, logger *zap.Logger) (*FileContentStore, error) {
	// TODO: Initialize BadgerDB at path
	// For now, use memory store
	logger.Info("file content store initialized (using memory backend)",
		zap.String("path", path),
	)

	return &FileContentStore{
		path:   path,
		logger: logger,
		mem:    NewMemoryContentStore(),
	}, nil
}

func (f *FileContentStore) Put(ctx context.Context, cid string, data []byte) error {
	return f.mem.Put(ctx, cid, data)
}

func (f *FileContentStore) Get(ctx context.Context, cid string) ([]byte, error) {
	return f.mem.Get(ctx, cid)
}

func (f *FileContentStore) Has(ctx context.Context, cid string) (bool, error) {
	return f.mem.Has(ctx, cid)
}

func (f *FileContentStore) Delete(ctx context.Context, cid string) error {
	return f.mem.Delete(ctx, cid)
}

func (f *FileContentStore) Close() error {
	// TODO: Close BadgerDB
	return f.mem.Close()
}
