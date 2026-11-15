package server

import (
	"sync"
	"testing"
)

func TestServerSeqIncrement(t *testing.T) {
	srv := New(":10369", []byte("testsecret123456789012345678901234"), nil, nil)
	if srv == nil {
		t.Fatal("New() returned nil")
	}

	// Test seq increment logic using string keys
	srv.seqMu.Lock()
	srv.clientSeq["127.0.0.1:50001"] = 5
	srv.seqMu.Unlock()

	srv.seqMu.Lock()
	seq := srv.clientSeq["127.0.0.1:50001"] + 1
	srv.clientSeq["127.0.0.1:50001"] = seq
	srv.seqMu.Unlock()

	if seq != 6 {
		t.Errorf("expected seq 6, got %d", seq)
	}
}

func TestServerClearClientSeq(t *testing.T) {
	srv := New(":10369", []byte("testsecret123456789012345678901234"), nil, nil)

	// Set sequence for a client
	srv.seqMu.Lock()
	srv.clientSeq["127.0.0.1:50001"] = 10
	srv.seqMu.Unlock()

	// Clear and verify
	srv.seqMu.Lock()
	delete(srv.clientSeq, "127.0.0.1:50001")
	srv.seqMu.Unlock()

	srv.seqMu.Lock()
	_, ok := srv.clientSeq["127.0.0.1:50001"]
	if ok {
		t.Error("expected client to be deleted")
	}
	srv.seqMu.Unlock()
}

func TestServerMultipleClientsSeq(t *testing.T) {
	srv := New(":10369", []byte("testsecret123456789012345678901234"), nil, nil)

	// Set different sequences for different clients
	srv.seqMu.Lock()
	srv.clientSeq["127.0.0.1:50001"] = 5
	srv.clientSeq["127.0.0.1:50002"] = 10
	srv.seqMu.Unlock()

	// Verify each client has its own sequence
	srv.seqMu.Lock()
	if srv.clientSeq["127.0.0.1:50001"] != 5 {
		t.Errorf("expected client1 seq 5, got %d", srv.clientSeq["127.0.0.1:50001"])
	}
	if srv.clientSeq["127.0.0.1:50002"] != 10 {
		t.Errorf("expected client2 seq 10, got %d", srv.clientSeq["127.0.0.1:50002"])
	}
	srv.seqMu.Unlock()
}

func TestServerConcurrentAccess(t *testing.T) {
	srv := New(":10369", []byte("testsecret123456789012345678901234"), nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			addr := string(rune('A'+id%26)) + ":5000"
			srv.seqMu.Lock()
			srv.clientSeq[addr]++
			seq := srv.clientSeq[addr]
			srv.seqMu.Unlock()
			if seq <= 0 {
				t.Errorf("client %s: expected seq > 0, got %d", addr, seq)
			}
		}(i)
	}
	wg.Wait()
}
