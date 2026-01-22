// internal/realtime/channel.go
package realtime

import (
	"sync"
)

// Channel represents a realtime channel with subscribers
type Channel struct {
	topic       string
	private     bool
	mu          sync.RWMutex
	subscribers map[string]*ChannelSub // connID -> subscription
	presence    *PresenceState         // nil if presence not enabled
}

// ChannelSub represents a connection's subscription to a channel
type ChannelSub struct {
	conn            *Conn
	joinRef         string
	broadcastConfig BroadcastConfig
	presenceConfig  PresenceConfig
	pgChanges       []PostgresChangeSub
}

// addSubscriber adds a subscription to the channel
func (ch *Channel) addSubscriber(connID string, sub *ChannelSub) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.subscribers[connID] = sub
}

// removeSubscriber removes a subscription from the channel
func (ch *Channel) removeSubscriber(connID string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	delete(ch.subscribers, connID)
}

// getSubscriber returns a subscriber by connection ID
func (ch *Channel) getSubscriber(connID string) *ChannelSub {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.subscribers[connID]
}

// getSubscribers returns all subscribers (snapshot)
func (ch *Channel) getSubscribers() []*ChannelSub {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	subs := make([]*ChannelSub, 0, len(ch.subscribers))
	for _, sub := range ch.subscribers {
		subs = append(subs, sub)
	}
	return subs
}

// isEmpty returns true if the channel has no subscribers
func (ch *Channel) isEmpty() bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return len(ch.subscribers) == 0
}

// enablePresence initializes presence state for the channel
func (ch *Channel) enablePresence() {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if ch.presence == nil {
		ch.presence = NewPresenceState()
	}
}

// getPresence returns the presence state (may be nil)
func (ch *Channel) getPresence() *PresenceState {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.presence
}
