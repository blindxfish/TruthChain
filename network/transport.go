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

// Sync transport bounds so a peer cannot hang a sync goroutine or force
// unbounded buffering with a request/response that never sends a newline.
const (
	syncRequestMaxBytes  = 64 * 1024        // a ChainSyncRequest is small
	syncResponseMaxBytes = 32 * 1024 * 1024 // a block batch can be large
	syncReadDeadline     = 60 * time.Second
	syncWriteDeadline    = 60 * time.Second
)

// StartSyncServer starts a TCP server to handle chain sync requests
func StartSyncServer(bindAddr string, bc *blockchain.Blockchain, nodeID string) error {
	fmt.Printf("[SyncServer] Attempting to bind to %s...\n", bindAddr)
	ln, err := net.Listen("tcp", bindAddr)
	if err != nil {
		fmt.Printf("[SyncServer] ERROR: Failed to bind to %s: %v\n", bindAddr, err)
		return fmt.Errorf("failed to start sync server: %w", err)
	}
	fmt.Printf("[SyncServer] SUCCESS: Listening on %s\n", bindAddr)
	fmt.Printf("[SyncServer] Ready to accept sync connections on %s\n", bindAddr)
	defer func() {
		fmt.Printf("[SyncServer] Shutting down sync server on %s\n", bindAddr)
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("[SyncServer] Accept error: %v\n", err)
			continue
		}
		fmt.Printf("[SyncServer] Accepted connection from %s\n", conn.RemoteAddr())
		go handleSyncConnection(conn, bc, nodeID)
	}
}

func handleSyncConnection(conn net.Conn, bc *blockchain.Blockchain, nodeID string) {
	defer conn.Close()

	// Bound the request read: deadline + size cap against slowloris / unbounded
	// buffering from a peer that never terminates its request line.
	_ = conn.SetReadDeadline(time.Now().Add(syncReadDeadline))
	reader := bufio.NewReader(io.LimitReader(conn, syncRequestMaxBytes))
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

	var resp chain.ChainSyncResponse
	resp.FromIndex = from
	resp.ToIndex = to
	resp.NodeID = nodeID
	resp.Timestamp = time.Now().Unix()

	if req.HeadersOnly {
		// Header-only response
		var headers []*chain.BlockHeader
		for i := from; i <= to; i++ {
			block, err := bc.GetBlockByIndex(i)
			if err == nil && block != nil {
				header := &chain.BlockHeader{
					Index:     block.Index,
					Timestamp: block.Timestamp,
					PrevHash:  block.PrevHash,
					Hash:      block.Hash,
					CharCount: block.CharCount,
					PostCount: block.GetPostCount(),
				}
				headers = append(headers, header)
			}
		}
		resp.Headers = headers
	} else {
		// Full block response
		var blocks []*chain.Block
		for i := from; i <= to; i++ {
			block, err := bc.GetBlockByIndex(i)
			if err == nil && block != nil {
				blocks = append(blocks, block)
			}
		}
		resp.Blocks = blocks
	}
	respBytes, _ := json.Marshal(resp)
	_ = conn.SetWriteDeadline(time.Now().Add(syncWriteDeadline))
	writer.Write(respBytes)
	writer.WriteByte('\n')
	writer.Flush()
	fmt.Printf("[SyncServer] Served sync request from %s: blocks %d-%d\n", req.NodeID, from, to)
}

// SyncFromPeerTCP connects to a peer and requests blocks via TCP
func SyncFromPeerTCP(peerAddr string, fromIndex int, toIndex int, nodeID string) (*chain.ChainSyncResponse, error) {
	return SyncFromPeerTCPWithHeaders(peerAddr, fromIndex, toIndex, nodeID, false)
}

// SyncFromPeerTCPWithHeaders connects to a peer and requests blocks or headers via TCP
func SyncFromPeerTCPWithHeaders(peerAddr string, fromIndex int, toIndex int, nodeID string, headersOnly bool) (*chain.ChainSyncResponse, error) {
	conn, err := net.DialTimeout("tcp", peerAddr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peer %s: %w", peerAddr, err)
	}
	defer conn.Close()
	writer := bufio.NewWriter(conn)
	// Bound the response read: deadline + size cap so a malicious peer cannot
	// stream unbounded data or stall this sync forever.
	_ = conn.SetReadDeadline(time.Now().Add(syncReadDeadline))
	reader := bufio.NewReader(io.LimitReader(conn, syncResponseMaxBytes))

	// Send request
	req := chain.ChainSyncRequest{
		FromIndex:   fromIndex,
		ToIndex:     toIndex,
		NodeID:      nodeID,
		Timestamp:   time.Now().Unix(),
		HeadersOnly: headersOnly,
	}
	reqBytes, _ := json.Marshal(req)
	_ = conn.SetWriteDeadline(time.Now().Add(syncWriteDeadline))
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
