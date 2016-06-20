package events

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/setting"
)

type Event interface {
	Type() string
	Timestamp() time.Time
	Body() ([]byte, error)
}

type RawEvent struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Body      json.RawMessage `json:"payload"`
	Source    string          `json:"source"`
	Attempts  int             `json:"attempts"`
}

func NewRawEventFromEvent(e Event) (*RawEvent, error) {
	payload, err := e.Body()
	if err != nil {
		return nil, err
	}
	hostname, _ := os.Hostname()
	raw := &RawEvent{
		Type:      e.Type(),
		Timestamp: e.Timestamp(),
		Source:    hostname,
		Body:      payload,
	}
	return raw, nil
}

type Handlers struct {
	sync.Mutex
	Listeners map[string][]chan<- RawEvent
}

func (h *Handlers) Add(key string, ch chan<- RawEvent) {
	h.Lock()
	if l, ok := h.Listeners[key]; !ok {
		l = make([]chan<- RawEvent, 0)
		h.Listeners[key] = l
	}
	h.Listeners[key] = append(h.Listeners[key], ch)
	h.Unlock()
}

func (h *Handlers) GetListeners(key string) []chan<- RawEvent {
	listeners := make([]chan<- RawEvent, 0)
	h.Lock()
	for rk, l := range h.Listeners {
		if rk == "*" || rk == key {
			listeners = append(listeners, l...)
		}
	}
	h.Unlock()
	return listeners
}

var (
	handlers *Handlers
	pubChan  chan Message
	subChan  chan Message
)

func Init() {
	handlers = &Handlers{
		Listeners: make(map[string][]chan<- RawEvent),
	}
	pubChan = make(chan Message, 100)

	if setting.Rabbitmq.Enabled {
		// use rabbitmq for message distribution.
		subChan = make(chan Message, 10)
		go Run(setting.Rabbitmq.Url, setting.Rabbitmq.EventsExchange, pubChan, subChan)
		go handleMessages(subChan)
	} else {
		// handle all message written to the publish chan.
		go handleMessages(pubChan)
	}
	return
}

func Subscribe(t string, channel chan<- RawEvent) {
	handlers.Add(t, channel)
}

func Publish(e Event, attempts int) error {
	if handlers == nil {
		// not initialized.
		return nil
	}
	raw, err := NewRawEventFromEvent(e)
	if err != nil {
		return err
	}
	raw.Attempts = attempts + 1

	body, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	msg := Message{
		RoutingKey: e.Type(),
		Payload:    body,
	}
	ticker := time.NewTicker(2 * time.Second)
	pre := time.Now()
WAITLOOP:
	for {
		select {
		case <-ticker.C:
			log.Error(3, "blocked writing to event publish channel for %f seconds", time.Since(pre).Seconds())
		case pubChan <- msg:
			ticker.Stop()
			break WAITLOOP
		}
	}

	return nil
}

func handleMessages(c chan Message) {
	for m := range c {
		go func(msg Message) {
			e := RawEvent{}
			err := json.Unmarshal(msg.Payload, &e)
			if err != nil {
				log.Error(3, "unable to unmarshal event Message. %s", err)
				return
			}

			log.Debug("processing event of type %s", e.Type)
			//broadcast the event to listeners.
			for _, ch := range handlers.GetListeners(e.Type) {
				ch <- e
			}
		}(m)
	}
}
