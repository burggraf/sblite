// internal/realtime/channel_test.go
package realtime

import (
	"testing"
)

func TestChannelAddSubscriber(t *testing.T) {
	ch := &Channel{
		topic:       "realtime:test",
		subscribers: make(map[string]*ChannelSub),
	}

	sub := &ChannelSub{
		joinRef: "1",
		broadcastConfig: BroadcastConfig{
			Ack:  true,
			Self: false,
		},
	}

	ch.addSubscriber("conn-1", sub)

	if len(ch.subscribers) != 1 {
		t.Errorf("expected 1 subscriber, got %d", len(ch.subscribers))
	}

	if ch.subscribers["conn-1"] != sub {
		t.Error("subscriber not added correctly")
	}
}

func TestChannelRemoveSubscriber(t *testing.T) {
	ch := &Channel{
		topic:       "realtime:test",
		subscribers: make(map[string]*ChannelSub),
	}

	sub := &ChannelSub{joinRef: "1"}
	ch.addSubscriber("conn-1", sub)
	ch.removeSubscriber("conn-1")

	if len(ch.subscribers) != 0 {
		t.Errorf("expected 0 subscribers, got %d", len(ch.subscribers))
	}
}

func TestChannelGetSubscribers(t *testing.T) {
	ch := &Channel{
		topic:       "realtime:test",
		subscribers: make(map[string]*ChannelSub),
	}

	sub1 := &ChannelSub{joinRef: "1"}
	sub2 := &ChannelSub{joinRef: "2"}
	ch.addSubscriber("conn-1", sub1)
	ch.addSubscriber("conn-2", sub2)

	subs := ch.getSubscribers()
	if len(subs) != 2 {
		t.Errorf("expected 2 subscribers, got %d", len(subs))
	}
}

func TestChannelGetSubscriber(t *testing.T) {
	ch := &Channel{
		topic:       "realtime:test",
		subscribers: make(map[string]*ChannelSub),
	}

	sub := &ChannelSub{joinRef: "1"}
	ch.addSubscriber("conn-1", sub)

	got := ch.getSubscriber("conn-1")
	if got != sub {
		t.Error("expected to get the added subscriber")
	}

	notFound := ch.getSubscriber("conn-999")
	if notFound != nil {
		t.Error("expected nil for non-existent subscriber")
	}
}

func TestChannelIsEmpty(t *testing.T) {
	ch := &Channel{
		topic:       "realtime:test",
		subscribers: make(map[string]*ChannelSub),
	}

	if !ch.isEmpty() {
		t.Error("expected new channel to be empty")
	}

	ch.addSubscriber("conn-1", &ChannelSub{joinRef: "1"})

	if ch.isEmpty() {
		t.Error("expected channel with subscriber to not be empty")
	}

	ch.removeSubscriber("conn-1")

	if !ch.isEmpty() {
		t.Error("expected channel to be empty after removing subscriber")
	}
}

func TestChannelEnablePresence(t *testing.T) {
	ch := &Channel{
		topic:       "realtime:test",
		subscribers: make(map[string]*ChannelSub),
	}

	if ch.getPresence() != nil {
		t.Error("expected presence to be nil initially")
	}

	ch.enablePresence()

	if ch.getPresence() == nil {
		t.Error("expected presence to be non-nil after enabling")
	}

	// Calling enablePresence again should not create a new instance
	presence1 := ch.getPresence()
	ch.enablePresence()
	presence2 := ch.getPresence()

	if presence1 != presence2 {
		t.Error("expected enablePresence to be idempotent")
	}
}
