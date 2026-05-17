package daemon

import (
	"encoding/json"
	"sync"
)

// Event is what we broadcast to UI clients.
type Event struct {
	Type string `json:"type"`           // "list_updated", "list_deleted", "ping"
	Slug string `json:"slug,omitempty"`
}

// broker is a tiny pub/sub. Subscribers receive events on a buffered channel.
type broker struct {
	mu      sync.Mutex
	subs    map[chan Event]bool
	nextSub chan Event
}

func newBroker() *broker {
	return &broker{subs: map[chan Event]bool{}}
}

func (b *broker) subscribe() chan Event {
	ch := make(chan Event, 8)
	b.mu.Lock()
	b.subs[ch] = true
	b.mu.Unlock()
	return ch
}

func (b *broker) unsubscribe(ch chan Event) {
	b.mu.Lock()
	if b.subs[ch] {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
}

func (b *broker) publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
			// Slow subscriber; drop. UI will recover on next event.
		}
	}
}

func (e Event) marshal() []byte {
	b, _ := json.Marshal(e)
	return b
}
