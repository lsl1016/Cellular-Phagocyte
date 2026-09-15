package main

import (
	"math/rand"
	"testing"
	"time"
)

func TestPercentileMs(t *testing.T) {
	values := []time.Duration{10 * time.Millisecond, 40 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	if got := percentileMs(values, 0.50); got != 20 {
		t.Fatalf("p50 = %.1fms, want 20ms", got)
	}
	if got := percentileMs(values, 0.95); got != 40 {
		t.Fatalf("p95 = %.1fms, want 40ms", got)
	}
}

func TestSelectClientsUsesRequestedFraction(t *testing.T) {
	clients := make([]*loadClient, 50)
	for i := range clients {
		clients[i] = &loadClient{index: i}
	}
	selected := selectClients(clients, 0.30, rand.New(rand.NewSource(1)))
	if len(selected) != 15 {
		t.Fatalf("selected %d clients, want 15", len(selected))
	}
	seen := make(map[int]struct{}, len(selected))
	for _, c := range selected {
		if _, ok := seen[c.index]; ok {
			t.Fatalf("client %d selected twice", c.index)
		}
		seen[c.index] = struct{}{}
	}
}
