package daemon

import (
	"testing"
	"time"
)

func TestSSEHubSubscribeAndBroadcast(t *testing.T) {
	hub := NewSSEHub()
	ch, unsub := hub.Subscribe("s1")
	defer unsub()

	evt := DaemonEvent{
		Timestamp:   time.Now().Format(time.RFC3339),
		Source:      "proxy",
		EventType:   "api_request",
		SessionID:   "s1",
		Description: "test event",
	}
	hub.Broadcast("s1", evt)

	select {
	case got := <-ch:
		if got.Description != "test event" {
			t.Errorf("Description = %q", got.Description)
		}
		if got.SessionID != "s1" {
			t.Errorf("SessionID = %q", got.SessionID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestSSEHubBroadcastNoSubscribers(t *testing.T) {
	hub := NewSSEHub()
	// Should not panic
	hub.Broadcast("s1", DaemonEvent{Description: "no one listening"})
}

func TestSSEHubUnsubscribe(t *testing.T) {
	hub := NewSSEHub()
	_, unsub := hub.Subscribe("s1")

	if hub.ClientCount("s1") != 1 {
		t.Errorf("client count = %d, want 1", hub.ClientCount("s1"))
	}

	unsub()

	if hub.ClientCount("s1") != 0 {
		t.Errorf("client count after unsub = %d, want 0", hub.ClientCount("s1"))
	}
}

func TestSSEHubMultipleSubscribers(t *testing.T) {
	hub := NewSSEHub()
	ch1, unsub1 := hub.Subscribe("s1")
	ch2, unsub2 := hub.Subscribe("s1")
	defer unsub1()
	defer unsub2()

	hub.Broadcast("s1", DaemonEvent{Description: "multi"})

	select {
	case e := <-ch1:
		if e.Description != "multi" {
			t.Errorf("ch1 Description = %q", e.Description)
		}
	case <-time.After(time.Second):
		t.Fatal("ch1 timeout")
	}

	select {
	case e := <-ch2:
		if e.Description != "multi" {
			t.Errorf("ch2 Description = %q", e.Description)
		}
	case <-time.After(time.Second):
		t.Fatal("ch2 timeout")
	}
}

func TestSSEHubDifferentSessions(t *testing.T) {
	hub := NewSSEHub()
	ch1, unsub1 := hub.Subscribe("s1")
	ch2, unsub2 := hub.Subscribe("s2")
	defer unsub1()
	defer unsub2()

	hub.Broadcast("s1", DaemonEvent{Description: "for s1"})

	select {
	case <-ch1:
		// expected
	case <-time.After(time.Second):
		t.Fatal("ch1 should have received event")
	}

	// ch2 should NOT receive the event
	select {
	case e := <-ch2:
		t.Fatalf("ch2 should not receive event, got: %v", e)
	case <-time.After(50 * time.Millisecond):
		// expected — no event
	}
}

func TestSSEHubClientCount(t *testing.T) {
	hub := NewSSEHub()
	if hub.ClientCount("s1") != 0 {
		t.Errorf("empty hub count = %d", hub.ClientCount("s1"))
	}

	_, unsub1 := hub.Subscribe("s1")
	_, unsub2 := hub.Subscribe("s1")
	if hub.ClientCount("s1") != 2 {
		t.Errorf("two clients count = %d", hub.ClientCount("s1"))
	}

	unsub1()
	if hub.ClientCount("s1") != 1 {
		t.Errorf("after one unsub count = %d", hub.ClientCount("s1"))
	}

	unsub2()
	if hub.ClientCount("s1") != 0 {
		t.Errorf("after all unsub count = %d", hub.ClientCount("s1"))
	}
}
