package network

import (
	"encoding/json"
	"testing"
)

func TestJSONObjectEnd(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int // expected end index, or -1 if incomplete
	}{
		{"simple object", `{"a":1}`, 7},
		{"trailing data", `{"a":1}garbage`, 7},
		{"nested", `{"a":{"b":[1,2]}}`, 17},
		{"brace inside string", `{"a":"}"}`, 9},
		{"escaped quote in string", `{"a":"x\"}y"}`, 13},
		{"array", `[1,2,3]`, 7},
		{"incomplete", `{"a":1`, -1},
		{"incomplete nested", `{"a":{"b":1}`, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := jsonObjectEnd([]byte(c.in), 0); got != c.want {
				t.Errorf("jsonObjectEnd(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

// TestProcessStreamReassembly proves that messages split across reads, and
// concatenated with PING tokens, are correctly reassembled and delivered — the
// core bug the old fixed-4KB single-read parser could not handle.
func TestProcessStreamReassembly(t *testing.T) {
	mm := &MeshManager{network: &TrustNetwork{MessageChan: make(chan NetworkMessage, 16)}}

	marshal := func(src string) []byte {
		b, err := json.Marshal(NetworkMessage{Type: MessageTypePost, Source: src, Timestamp: 1, TTL: 5})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return b
	}

	// Stream: msg1, a bare PING token, msg2, msg3 (no framing between them).
	var full []byte
	full = append(full, marshal("node-1")...)
	full = append(full, []byte("PING:123456")...)
	full = append(full, marshal("node-2")...)
	full = append(full, marshal("node-3")...)

	// Feed the stream in tiny 5-byte chunks, mimicking arbitrary TCP segmentation,
	// running the same accumulate/consume loop handleConnection uses.
	var acc []byte
	for i := 0; i < len(full); i += 5 {
		end := i + 5
		if end > len(full) {
			end = len(full)
		}
		acc = append(acc, full[i:end]...)
		consumed, _ := mm.processStream("test-peer", acc)
		acc = append(acc[:0], acc[consumed:]...)
	}

	if len(acc) != 0 {
		t.Errorf("expected all bytes consumed, %d left over: %q", len(acc), string(acc))
	}

	got := map[string]bool{}
	for len(mm.network.MessageChan) > 0 {
		msg := <-mm.network.MessageChan
		got[msg.Source] = true
	}
	for _, want := range []string{"node-1", "node-2", "node-3"} {
		if !got[want] {
			t.Errorf("message from %s was not delivered (reassembly lost it)", want)
		}
	}
	if len(got) != 3 {
		t.Errorf("expected 3 delivered messages, got %d: %v", len(got), got)
	}
}

// TestProcessStreamRetainsIncompleteTail verifies a partial trailing object is
// held back (not dropped) until the rest arrives.
func TestProcessStreamRetainsIncompleteTail(t *testing.T) {
	mm := &MeshManager{network: &TrustNetwork{MessageChan: make(chan NetworkMessage, 4)}}
	msg, _ := json.Marshal(NetworkMessage{Type: MessageTypePost, Source: "n", Timestamp: 1, TTL: 1})

	// First half of the object: nothing should be delivered, nothing consumed.
	half := msg[:len(msg)/2]
	consumed, processed := mm.processStream("p", half)
	if processed != 0 {
		t.Fatalf("partial message should not be processed, got %d", processed)
	}
	if consumed != 0 {
		t.Fatalf("partial message start should be retained (consumed 0), got %d", consumed)
	}

	// Now the full object arrives: it should be delivered.
	consumed, processed = mm.processStream("p", msg)
	if processed != 1 {
		t.Fatalf("expected 1 processed once complete, got %d", processed)
	}
	if consumed != len(msg) {
		t.Fatalf("expected full consume %d, got %d", len(msg), consumed)
	}
}
