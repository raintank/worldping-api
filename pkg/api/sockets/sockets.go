package sockets

import (
	"math/rand"
	"sync"
	"time"

	"github.com/grafana/metrictank/stats"
	"github.com/raintank/worldping-api/pkg/log"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services"
)

var (
	ProbesConnected = stats.NewGauge32("api.probes.connected")
	UpdatesSent     = stats.NewCounter32("api.probes.updates-sent")
	CreatesSent     = stats.NewCounter32("api.probes.creates-sent")
	RemovesSent     = stats.NewCounter32("api.probes.removes-sent")

	socketCache *Cache
	publisher   services.MetricsEventsPublisher
)

type Cache struct {
	sync.RWMutex
	Sockets     map[string]*ProbeSocket
	done        chan struct{}
	refreshChan chan int64
}

func InitCache(pub services.MetricsEventsPublisher) {
	publisher = pub
	if socketCache != nil {
		return
	}
	socketCache = &Cache{
		Sockets:     make(map[string]*ProbeSocket),
		done:        make(chan struct{}),
		refreshChan: make(chan int64, 100),
	}

	go socketCache.refreshLoop()
	go socketCache.refreshQueue()
}

func Shutdown() {
	socketCache.Shutdown()
}

func Set(id string, sock *ProbeSocket) {
	socketCache.Set(id, sock)
}

func Remove(id string) {
	socketCache.Remove(id)
}

func Emit(id string, event string, payload interface{}) {
	socketCache.Emit(id, event, payload)
}

func Refresh(id int64) {
	socketCache.Refresh(id)
}

func UpdateProbe(probe *m.ProbeDTO) {
	socketCache.UpdateProbe(probe)
}

func (c *Cache) Set(id string, sock *ProbeSocket) {
	c.Lock()
	c.Sockets[id] = sock
	c.Unlock()
	ProbesConnected.Inc()
}

func (c *Cache) Remove(id string) {
	c.Lock()
	delete(c.Sockets, id)
	c.Unlock()
	ProbesConnected.Dec()
}

func (c *Cache) Shutdown() {
	c.done <- struct{}{}
	sessList := make([]*ProbeSocket, 0)
	c.Lock()
	for _, sock := range c.Sockets {
		sessList = append(sessList, sock)
	}
	c.Sockets = make(map[string]*ProbeSocket)
	c.Unlock()
	for _, sock := range sessList {
		sock.Remove()
	}
	return
}

func (c *Cache) Emit(id string, event string, payload interface{}) {
	c.RLock()
	socket, ok := c.Sockets[id]
	if !ok {
		log.Info("socket " + id + " is not local.")
		c.RUnlock()
		return
	}
	c.RUnlock()
	socket.emit(event, payload)
	switch event {
	case "updated":
		UpdatesSent.Inc()
	case "created":
		CreatesSent.Inc()
	case "removed":
		RemovesSent.Inc()
	}
}

func (c *Cache) Refresh(id int64) {
	c.refreshChan <- id
}

func (c *Cache) refresh(id int64) {
	sessList := make([]*ProbeSocket, 0)
	c.RLock()
	for _, sock := range c.Sockets {
		if sock.Probe.Id == id {
			sessList = append(sessList, sock)
		}
	}
	c.RUnlock()
	for _, sock := range sessList {
		sock.Refresh()
	}
}

func (c *Cache) refreshQueue() {
	ticker := time.NewTicker(time.Second * 2)
	buffer := make([]int64, 0)
	for {
		select {
		case <-c.done:
			log.Info("RefreshQueue terminating due to shutdown signal.")
			ticker.Stop()
			return
		case <-ticker.C:
			if len(buffer) == 0 {
				break
			}
			log.Debug("processing %d queued probe refreshes.", len(buffer))
			ids := make(map[int64]struct{})
			for _, id := range buffer {
				ids[id] = struct{}{}
			}
			log.Debug("%d refreshes are for %d probes", len(buffer), len(ids))
			for id := range ids {
				c.refresh(id)
			}
			buffer = buffer[:0]
		case id := <-c.refreshChan:
			log.Debug("adding refresh of %d to buffer", id)
			buffer = append(buffer, id)
		}
	}
}

func (c *Cache) refreshLoop() {
	ticker := time.NewTicker(time.Minute)
	for {
		select {
		case <-c.done:
			log.Info("RefreshLoop terminating due to shutdown signal.")
			ticker.Stop()
			return
		case <-ticker.C:
			sessList := make([]*ProbeSocket, 0)
			c.RLock()
			for _, sock := range c.Sockets {
				// add some jitter so that we avoid all probes refreshing at the same time.
				// Probes will refresh between every 5 and 15minutes.
				maxRefreshDelay := time.Minute * time.Duration(5+rand.Intn(10))
				sock.Lock()
				lastRefresh := sock.LastRefresh
				sock.Unlock()
				if time.Since(lastRefresh) >= maxRefreshDelay {
					sessList = append(sessList, sock)
				}
			}
			c.RUnlock()
			for _, sock := range sessList {
				sock.Refresh()
			}
		}
	}
}

func (c *Cache) UpdateProbe(probe *m.ProbeDTO) {
	c.RLock()
	defer c.RUnlock()
	// get list of local sockets for this collector.
	sockets := make([]*ProbeSocket, 0)
	for _, sock := range c.Sockets {
		if sock.Probe.Id == probe.Id {
			sockets = append(sockets, sock)
		}
	}
	if len(sockets) > 0 {
		for _, sock := range sockets {
			sock.Probe = probe
			sock.EmitReady()
		}
	}
}
