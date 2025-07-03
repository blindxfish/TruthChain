package store

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/blindxfish/truthchain/chain"
	"go.etcd.io/bbolt"
)

// Storage interface defines the methods for persistent storage
type Storage interface {
	// Block operations
	SaveBlock(block *chain.Block) error
	GetBlock(index int) (*chain.Block, error)
	GetBlockByHash(hash string) (*chain.Block, error)
	GetLatestBlock() (*chain.Block, error)
	GetBlockCount() (int, error)
	DeleteBlock(index int) error

	// Post operations
	SavePost(post chain.Post) error
	GetPost(hash string) (*chain.Post, error)
	PostExists(hash string) (bool, error)

	// Pending posts operations
	SavePendingPost(post chain.Post) error
	GetPendingPosts() ([]chain.Post, error)
	RemovePendingPost(hash string) error
	ClearPendingPosts() error

	// Character balance operations
	GetCharacterBalance(address string) (int, error)
	UpdateCharacterBalance(address string, amount int) error

	// Heartbeat operations
	SaveHeartbeat(heartbeat []byte) error
	GetHeartbeats() ([][]byte, error)

	// Utility operations
	Close() error
}

// BoltDBStorage implements Storage interface using BoltDB
type BoltDBStorage struct {
	db   *bbolt.DB
	path string
	mu   sync.RWMutex
}

// Bucket names for organizing data
var (
	blocksBucket       = []byte("blocks")
	postsBucket        = []byte("posts")
	pendingPostsBucket = []byte("pending_posts")
	balancesBucket     = []byte("balances")
	metadataBucket     = []byte("metadata")
	heartbeatsBucket   = []byte("heartbeats")
)

// NewBoltDBStorage creates a new BoltDB storage instance
func NewBoltDBStorage(dbPath string) (*BoltDBStorage, error) {
	// Open database
	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &BoltDBStorage{
		db:   db,
		path: dbPath,
	}

	// Initialize buckets
	if err := storage.initializeBuckets(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return storage, nil
}

// initializeBuckets creates the necessary buckets if they don't exist
func (s *BoltDBStorage) initializeBuckets() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		buckets := [][]byte{blocksBucket, postsBucket, pendingPostsBucket, balancesBucket, metadataBucket, heartbeatsBucket}

		for _, bucketName := range buckets {
			_, err := tx.CreateBucketIfNotExists(bucketName)
			if err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
			}
		}

		return nil
	})
}

// SaveBlock saves a block to storage
func (s *BoltDBStorage) SaveBlock(block *chain.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		// Serialize block
		blockData, err := json.Marshal(block)
		if err != nil {
			return fmt.Errorf("failed to marshal block: %w", err)
		}

		// Save block by index
		blocksBucket := tx.Bucket(blocksBucket)
		indexKey := fmt.Sprintf("%d", block.Index)
		if err := blocksBucket.Put([]byte(indexKey), blockData); err != nil {
			return fmt.Errorf("failed to save block: %w", err)
		}

		// Save block by hash for quick lookup
		if err := blocksBucket.Put([]byte(block.Hash), blockData); err != nil {
			return fmt.Errorf("failed to save block by hash: %w", err)
		}

		// Update latest block index in metadata
		metadataBucket := tx.Bucket(metadataBucket)
		latestKey := []byte("latest_block_index")
		latestData := fmt.Sprintf("%d", block.Index)
		if err := metadataBucket.Put(latestKey, []byte(latestData)); err != nil {
			return fmt.Errorf("failed to update latest block index: %w", err)
		}

		return nil
	})
}

// GetBlock retrieves a block by index
func (s *BoltDBStorage) GetBlock(index int) (*chain.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var block *chain.Block
	err := s.db.View(func(tx *bbolt.Tx) error {
		blocksBucket := tx.Bucket(blocksBucket)
		indexKey := fmt.Sprintf("%d", index)
		blockData := blocksBucket.Get([]byte(indexKey))

		if blockData == nil {
			return fmt.Errorf("block not found: %d", index)
		}

		block = &chain.Block{}
		if err := json.Unmarshal(blockData, block); err != nil {
			return fmt.Errorf("failed to unmarshal block: %w", err)
		}

		return nil
	})

	return block, err
}

// GetBlockByHash retrieves a block by hash
func (s *BoltDBStorage) GetBlockByHash(hash string) (*chain.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var block *chain.Block
	err := s.db.View(func(tx *bbolt.Tx) error {
		blocksBucket := tx.Bucket(blocksBucket)
		blockData := blocksBucket.Get([]byte(hash))

		if blockData == nil {
			return fmt.Errorf("block not found: %s", hash)
		}

		block = &chain.Block{}
		if err := json.Unmarshal(blockData, block); err != nil {
			return fmt.Errorf("failed to unmarshal block: %w", err)
		}

		return nil
	})

	return block, err
}

// GetLatestBlock retrieves the most recent block
func (s *BoltDBStorage) GetLatestBlock() (*chain.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var block *chain.Block
	err := s.db.View(func(tx *bbolt.Tx) error {
		// Get latest block index from metadata
		metadataBucket := tx.Bucket(metadataBucket)
		latestKey := []byte("latest_block_index")
		latestData := metadataBucket.Get(latestKey)

		if latestData == nil {
			return fmt.Errorf("no blocks found")
		}

		// Get the latest block
		blocksBucket := tx.Bucket(blocksBucket)
		blockData := blocksBucket.Get(latestData)

		if blockData == nil {
			return fmt.Errorf("latest block not found")
		}

		block = &chain.Block{}
		if err := json.Unmarshal(blockData, block); err != nil {
			return fmt.Errorf("failed to unmarshal block: %w", err)
		}

		return nil
	})

	return block, err
}

// DeleteBlock deletes a block by index
func (s *BoltDBStorage) DeleteBlock(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		blocksBucket := tx.Bucket(blocksBucket)
		indexKey := fmt.Sprintf("%d", index)

		// Get the block first to get its hash
		blockData := blocksBucket.Get([]byte(indexKey))
		if blockData == nil {
			return fmt.Errorf("block not found: %d", index)
		}

		block := &chain.Block{}
		if err := json.Unmarshal(blockData, block); err != nil {
			return fmt.Errorf("failed to unmarshal block: %w", err)
		}

		// Delete by index
		if err := blocksBucket.Delete([]byte(indexKey)); err != nil {
			return fmt.Errorf("failed to delete block by index: %w", err)
		}

		// Delete by hash
		if err := blocksBucket.Delete([]byte(block.Hash)); err != nil {
			return fmt.Errorf("failed to delete block by hash: %w", err)
		}

		// Update latest block index if this was the latest
		metadataBucket := tx.Bucket(metadataBucket)
		latestKey := []byte("latest_block_index")
		latestData := metadataBucket.Get(latestKey)

		if latestData != nil {
			var latestIndex int
			if _, err := fmt.Sscanf(string(latestData), "%d", &latestIndex); err == nil {
				if latestIndex == index {
					// Simply set to previous index, or remove if it was the only block
					if index > 0 {
						newLatestData := fmt.Sprintf("%d", index-1)
						if err := metadataBucket.Put(latestKey, []byte(newLatestData)); err != nil {
							return fmt.Errorf("failed to update latest block index: %w", err)
						}
					} else {
						// This was the genesis block, remove the metadata
						if err := metadataBucket.Delete(latestKey); err != nil {
							return fmt.Errorf("failed to remove latest block index: %w", err)
						}
					}
				}
			}
		}

		return nil
	})
}

// GetBlockCount returns the total number of blocks
func (s *BoltDBStorage) GetBlockCount() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.View(func(tx *bbolt.Tx) error {
		metadataBucket := tx.Bucket(metadataBucket)
		latestKey := []byte("latest_block_index")
		latestData := metadataBucket.Get(latestKey)

		if latestData == nil {
			count = 0
			return nil
		}

		// Parse the latest block index and add 1 (since index is 0-based)
		var latestIndex int
		if _, err := fmt.Sscanf(string(latestData), "%d", &latestIndex); err != nil {
			return fmt.Errorf("failed to parse latest block index: %w", err)
		}

		count = latestIndex + 1
		return nil
	})

	return count, err
}

// SavePost saves a post to storage
func (s *BoltDBStorage) SavePost(post chain.Post) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		// Serialize post
		postData, err := json.Marshal(post)
		if err != nil {
			return fmt.Errorf("failed to marshal post: %w", err)
		}

		// Save post by hash
		postsBucket := tx.Bucket(postsBucket)
		if err := postsBucket.Put([]byte(post.Hash), postData); err != nil {
			return fmt.Errorf("failed to save post: %w", err)
		}

		return nil
	})
}

// GetPost retrieves a post by hash
func (s *BoltDBStorage) GetPost(hash string) (*chain.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var post *chain.Post
	err := s.db.View(func(tx *bbolt.Tx) error {
		postsBucket := tx.Bucket(postsBucket)
		postData := postsBucket.Get([]byte(hash))

		if postData == nil {
			return fmt.Errorf("post not found: %s", hash)
		}

		post = &chain.Post{}
		if err := json.Unmarshal(postData, post); err != nil {
			return fmt.Errorf("failed to unmarshal post: %w", err)
		}

		return nil
	})

	return post, err
}

// PostExists checks if a post exists by hash
func (s *BoltDBStorage) PostExists(hash string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var exists bool
	err := s.db.View(func(tx *bbolt.Tx) error {
		postsBucket := tx.Bucket(postsBucket)
		postData := postsBucket.Get([]byte(hash))
		exists = postData != nil
		return nil
	})

	return exists, err
}

// GetCharacterBalance retrieves the character balance for an address
func (s *BoltDBStorage) GetCharacterBalance(address string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var balance int
	err := s.db.View(func(tx *bbolt.Tx) error {
		balancesBucket := tx.Bucket(balancesBucket)
		balanceData := balancesBucket.Get([]byte(address))

		if balanceData == nil {
			balance = 0
			return nil
		}

		if _, err := fmt.Sscanf(string(balanceData), "%d", &balance); err != nil {
			return fmt.Errorf("failed to parse balance: %w", err)
		}

		return nil
	})

	return balance, err
}

// UpdateCharacterBalance updates the character balance for an address
func (s *BoltDBStorage) UpdateCharacterBalance(address string, amount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		balancesBucket := tx.Bucket(balancesBucket)

		// Get current balance
		currentBalanceData := balancesBucket.Get([]byte(address))
		currentBalance := 0

		if currentBalanceData != nil {
			if _, err := fmt.Sscanf(string(currentBalanceData), "%d", &currentBalance); err != nil {
				return fmt.Errorf("failed to parse current balance: %w", err)
			}
		}

		// Calculate new balance
		newBalance := currentBalance + amount
		if newBalance < 0 {
			return fmt.Errorf("insufficient balance: %d, trying to subtract %d", currentBalance, -amount)
		}

		// Save new balance
		newBalanceData := fmt.Sprintf("%d", newBalance)
		if err := balancesBucket.Put([]byte(address), []byte(newBalanceData)); err != nil {
			return fmt.Errorf("failed to update balance: %w", err)
		}

		return nil
	})
}

// SavePendingPost saves a post to the pending posts bucket
func (s *BoltDBStorage) SavePendingPost(post chain.Post) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		// Serialize post
		postData, err := json.Marshal(post)
		if err != nil {
			return fmt.Errorf("failed to marshal pending post: %w", err)
		}

		// Save post by hash
		pendingBucket := tx.Bucket(pendingPostsBucket)
		if err := pendingBucket.Put([]byte(post.Hash), postData); err != nil {
			return fmt.Errorf("failed to save pending post: %w", err)
		}

		return nil
	})
}

// GetPendingPosts retrieves all pending posts
func (s *BoltDBStorage) GetPendingPosts() ([]chain.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var posts []chain.Post
	err := s.db.View(func(tx *bbolt.Tx) error {
		pendingBucket := tx.Bucket(pendingPostsBucket)

		return pendingBucket.ForEach(func(key, value []byte) error {
			var post chain.Post
			if err := json.Unmarshal(value, &post); err != nil {
				return fmt.Errorf("failed to unmarshal pending post: %w", err)
			}
			posts = append(posts, post)
			return nil
		})
	})

	return posts, err
}

// RemovePendingPost removes a pending post by hash
func (s *BoltDBStorage) RemovePendingPost(hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		pendingBucket := tx.Bucket(pendingPostsBucket)
		return pendingBucket.Delete([]byte(hash))
	})
}

// ClearPendingPosts removes all pending posts
func (s *BoltDBStorage) ClearPendingPosts() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		pendingBucket := tx.Bucket(pendingPostsBucket)
		return pendingBucket.ForEach(func(key, value []byte) error {
			return pendingBucket.Delete(key)
		})
	})
}

// SaveHeartbeat saves a heartbeat to storage
func (s *BoltDBStorage) SaveHeartbeat(heartbeat []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bbolt.Tx) error {
		heartbeatsBucket := tx.Bucket(heartbeatsBucket)

		// Use timestamp as key for unique identification
		timestamp := fmt.Sprintf("%d", time.Now().UnixNano())

		if err := heartbeatsBucket.Put([]byte(timestamp), heartbeat); err != nil {
			return fmt.Errorf("failed to save heartbeat: %w", err)
		}

		return nil
	})
}

// GetHeartbeats retrieves all heartbeats from storage
func (s *BoltDBStorage) GetHeartbeats() ([][]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var heartbeats [][]byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		heartbeatsBucket := tx.Bucket(heartbeatsBucket)

		return heartbeatsBucket.ForEach(func(key, value []byte) error {
			// Copy the value to avoid issues with the transaction
			heartbeatCopy := make([]byte, len(value))
			copy(heartbeatCopy, value)
			heartbeats = append(heartbeats, heartbeatCopy)
			return nil
		})
	})

	return heartbeats, err
}

// Close closes the database connection
func (s *BoltDBStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}
