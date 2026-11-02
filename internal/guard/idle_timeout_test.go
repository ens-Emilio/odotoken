package guard

import (
	"testing"
	"time"
)

func TestIdleTracker(t *testing.T) {
	tracker := NewIdleTracker(100 * time.Millisecond)

	// unknown entity is idle
	if !tracker.IsIdle("unknown") {
		t.Error("expected unknown entity to be idle")
	}

	// record activity, should not be idle immediately
	tracker.RecordActivity("client-a")
	if tracker.IsIdle("client-a") {
		t.Error("expected client-a to be active after recording")
	}

	// wait for timeout
	time.Sleep(150 * time.Millisecond)
	if !tracker.IsIdle("client-a") {
		t.Error("expected client-a to be idle after timeout")
	}
}

func TestIdleTrackerPrune(t *testing.T) {
	tracker := NewIdleTracker(50 * time.Millisecond)
	tracker.RecordActivity("old")
	time.Sleep(100 * time.Millisecond)
	tracker.RecordActivity("fresh")

	pruned := tracker.Prune()
	if pruned != 1 {
		t.Errorf("Prune() = %d, want 1", pruned)
	}
	if tracker.IsIdle("fresh") {
		t.Error("expected fresh entity to remain after prune")
	}
}