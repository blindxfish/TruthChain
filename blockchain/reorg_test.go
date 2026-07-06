package blockchain

import (
	"path/filepath"
	"testing"

	"github.com/blindxfish/truthchain/chain"
	"github.com/blindxfish/truthchain/store"
)

func newTestBlockchain(t *testing.T) (*Blockchain, store.Storage) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "chain.db")
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	t.Cleanup(func() { storage.Close() })
	bc, err := NewBlockchain(storage, 5, "truthchain-local")
	if err != nil {
		t.Fatalf("new blockchain: %v", err)
	}
	return bc, storage
}

// emptyBlock builds a valid 0-post ("timer") block at index carrying the given
// wallet snapshot in its StateRoot.
func emptyBlock(index int, prevHash string, wallets []chain.WalletState) *chain.Block {
	sr := &chain.StateRoot{Wallets: wallets, BlockIndex: index}
	sr.SetHash()
	b := &chain.Block{
		Index:     index,
		Timestamp: 1700000000 + int64(index),
		PrevHash:  prevHash,
		Posts:     []chain.Post{},
		Transfers: []chain.Transfer{},
		StateRoot: sr,
		CharCount: 0,
	}
	b.SetHash()
	return b
}

func TestRollbackToBlockRestoresStateAndTruncates(t *testing.T) {
	bc, storage := newTestBlockchain(t)

	genesis := chain.CreateGenesisBlock()
	block1 := emptyBlock(1, genesis.Hash, []chain.WalletState{{Address: "A", Balance: 100}})
	block2 := emptyBlock(2, block1.Hash, []chain.WalletState{{Address: "A", Balance: 50}, {Address: "B", Balance: 50}})

	for _, b := range []*chain.Block{genesis, block1, block2} {
		if err := storage.SaveBlock(b); err != nil {
			t.Fatalf("save block %d: %v", b.Index, err)
		}
	}
	// State currently reflects block2.
	if err := bc.stateManager.LoadStateFromStateRoot(block2.StateRoot); err != nil {
		t.Fatalf("load state: %v", err)
	}

	// Roll back to block 1.
	if err := bc.rollbackToBlock(1); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// Block 2 must be gone and the tip must be block 1.
	if n, _ := storage.GetBlockCount(); n != 2 {
		t.Fatalf("expected chain length 2 after rollback, got %d", n)
	}
	if _, err := storage.GetBlock(2); err == nil {
		t.Fatal("block 2 should have been deleted")
	}

	// State must be restored to block 1's snapshot: A=100, B absent.
	if a, ok := bc.stateManager.GetWalletState("A"); !ok || a.Balance != 100 {
		t.Fatalf("expected A=100 after rollback, got %+v (ok=%v)", a, ok)
	}
	if _, ok := bc.stateManager.GetWalletState("B"); ok {
		t.Fatal("B should not exist after rollback to block 1")
	}
}

func TestIntegrateBlocksFromSyncRestoresState(t *testing.T) {
	bc, _ := newTestBlockchain(t)

	genesis := chain.CreateGenesisBlock()
	block1 := emptyBlock(1, genesis.Hash, []chain.WalletState{{Address: "A", Balance: 100}})
	block2 := emptyBlock(2, block1.Hash, []chain.WalletState{{Address: "A", Balance: 50}, {Address: "B", Balance: 50}})

	added, _, err := bc.IntegrateBlocksFromSync([]*chain.Block{genesis, block1, block2})
	if err != nil {
		t.Fatalf("integrate: %v", err)
	}
	if added != 3 {
		t.Fatalf("expected 3 blocks added, got %d", added)
	}

	// After sync, in-memory state must reflect the tip (block2) snapshot.
	if a, ok := bc.stateManager.GetWalletState("A"); !ok || a.Balance != 50 {
		t.Fatalf("expected A=50 after sync, got %+v (ok=%v)", a, ok)
	}
	if b, ok := bc.stateManager.GetWalletState("B"); !ok || b.Balance != 50 {
		t.Fatalf("expected B=50 after sync, got %+v (ok=%v)", b, ok)
	}
}
