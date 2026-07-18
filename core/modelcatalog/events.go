package modelcatalog

import "sync"

type RefreshEvent struct {
	Type   string `json:"type"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
	Search string `json:"search,omitempty"`
	Sort   string `json:"sort,omitempty"`
	MinFit string `json:"min_fit,omitempty"`
}

type RefreshNotifier interface {
	Subscribe() (<-chan RefreshEvent, func())
}

type refreshBroker struct {
	mu   sync.Mutex
	subs map[chan RefreshEvent]struct{}
}

func newRefreshBroker() *refreshBroker {
	return &refreshBroker{subs: map[chan RefreshEvent]struct{}{}}
}

func (b *refreshBroker) Subscribe() (<-chan RefreshEvent, func()) {
	ch := make(chan RefreshEvent, 4)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		close(ch)
		b.mu.Unlock()
	}
}

func (b *refreshBroker) publish(event RefreshEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- event:
		default:
		}
	}
}
