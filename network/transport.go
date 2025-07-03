package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/blindxfish/truthchain/blockchain"
	"github.com/blindxfish/truthchain/chain"
)

// StartSyncServer starts a TCP server to handle chain sync requests
func StartSyncServer(bindAddr string, bc *blockchain.Blockchain, nodeID string) error {
	ln, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return fmt.Errorf("failed to start sync server: %w", err)
	}
	fmt.Printf("[SyncServer] Listening on %s\n", bindAddr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("[SyncServer] Accept error: %v\n", err)
			continue
		}
		go handleSyncConnection(conn, bc, nodeID)
	}
}

func handleSyncConnection(conn net.Conn, bc *blockchain.Blockchain, nodeID string) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read request
	line, err := reader.ReadBytes('\n')
	if err != nil {
		if err != io.EOF {
			fmt.Printf("[SyncServer] Read error: %v\n", err)
		}
		return
	}

	var req chain.ChainSyncRequest
	if err := json.Unmarshal(line, &req); err != nil {
		fmt.Printf("[SyncServer] Invalid request: %v\n", err)
		return
	}

	// Prepare response
	from := req.FromIndex
	to := req.ToIndex
	if to < 0 || to < from {
		chainLength, _ := bc.GetChainLength()
		to = chainLength - 1
	}
	var blocks []*chain.Block
	for i := from; i <= to; i++ {
		block, err := bc.GetBlockByIndex(i)
		if err == nil && block != nil {
			blocks = append(blocks, block)
		}
	}
	resp := chain.ChainSyncResponse{
		Blocks:    blocks,
		FromIndex: from,
		ToIndex:   to,
		NodeID:    nodeID,
		Timestamp: time.Now().Unix(),
	}
	respBytes, _ := json.Marshal(resp)
	writer.Write(respBytes)
	writer.WriteByte('\n')
	writer.Flush()
	fmt.Printf("[SyncServer] Served sync request from %s: blocks %d-%d\n", req.NodeID, from, to)
}

// SyncFromPeerTCP connects to a peer and requests blocks via TCP
func SyncFromPeerTCP(peerAddr string, fromIndex int, toIndex int, nodeID string) (*chain.ChainSyncResponse, error) {
	conn, err := net.DialTimeout("tcp", peerAddr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peer %s: %w", peerAddr, err)
	}
	defer conn.Close()
	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)

	// Send request
	req := chain.ChainSyncRequest{
		FromIndex: fromIndex,
		ToIndex:   toIndex,
		NodeID:    nodeID,
		Timestamp: time.Now().Unix(),
	}
	reqBytes, _ := json.Marshal(req)
	writer.Write(reqBytes)
	writer.WriteByte('\n')
	writer.Flush()

	// Read response
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	var resp chain.ChainSyncResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}
	return &resp, nil
}
