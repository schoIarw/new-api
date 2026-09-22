package middleware

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestPhoneMemoryIndependentScopeAndReservation(t *testing.T) {
	const workers = 50
	var wg sync.WaitGroup
	var granted atomic.Int32
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if acquirePhoneMemory("phone-group1-unique", 123456, [2]int{10, 5}) == 0 {
				granted.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := granted.Load(); got != 5 {
		t.Fatalf("granted %d, want 5 reserved completion slots", got)
	}
	if verdict := acquirePhoneMemory("phone-group2-unique", 123456, [2]int{10, 5}); verdict != 0 {
		t.Fatalf("a different token group shared counters: %d", verdict)
	}
	releasePhoneMemory("phone-group1-unique", 123456)
	if verdict := acquirePhoneMemory("phone-group1-unique", 123456, [2]int{10, 5}); verdict != 0 {
		t.Fatalf("failed request must free completion slot: %d", verdict)
	}
	if verdict := acquirePhoneMemory("phone-group1-unique", 123457, [2]int{10, 5}); verdict != 0 {
		t.Fatalf("new window must reset counters: %d", verdict)
	}
}

func TestPhoneMemoryRequestOnly(t *testing.T) {
	for i := 0; i < 3; i++ {
		if got := acquirePhoneMemory("phone-request-only", 923456, [2]int{3, 0}); got != 0 {
			t.Fatal(got)
		}
	}
	if got := acquirePhoneMemory("phone-request-only", 923456, [2]int{3, 0}); got != 1 {
		t.Fatalf("request-only limit was bypassed: %d", got)
	}
}

func TestUserIdentifierCounterScopeIsolation(t *testing.T) {
	shared := identifierCounterScope("group1", "prefix:13701010")
	if shared != identifierCounterScope("group1", "prefix:13701010") {
		t.Fatal("same prefix must share counter")
	}
	if shared == identifierCounterScope("group2", "prefix:13701010") {
		t.Fatal("group counters collided")
	}
	if shared == identifierCounterScope("group1", "identifier:13701010") {
		t.Fatal("default and special counters collided")
	}
	if identifierCounterScope("group1", "identifier:alice|app1") == identifierCounterScope("group1", "identifier:alice|app2") {
		t.Fatal("distinct full identifiers collided")
	}
	if identifierCounterScope("group1", "identifier:13701010202|appid|ip") == shared {
		t.Fatal("full identifier unexpectedly shared prefix counter")
	}
}
