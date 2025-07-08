// Copyright (c) 2021 ava
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

package im

import (
	"github.com/aiakit/ava/_example/tutorial/proto/pim"
	"sync"
)

var defaultHub *hub

func init() {
	defaultHub = &hub{
		lock:      new(sync.RWMutex),
		clients:   make(map[string]*point),
		broadCast: make(chan *pim.BroadcastMessage),
	}
	go defaultHub.poller()
}

type hub struct {
	lock      *sync.RWMutex
	clients   map[string]*point
	broadCast chan *pim.BroadcastMessage
	exit      chan struct{}
}

type point struct {
	name    string
	message chan *pim.BroadcastMessage
}

func (h *hub) registerClient(p *point) {
	if _, ok := h.clients[p.name]; !ok {
		h.lock.RLock()
		h.clients[p.name] = p
		h.lock.RUnlock()
	}
}

func (h *hub) unregisterClient(p *point) {
	if _, ok := h.clients[p.name]; ok {
		h.lock.RLock()
		delete(h.clients, p.name)
		p = nil
		h.lock.RUnlock()
	}
	h.exit <- struct{}{}
}

func (h *hub) poller() {

	for {
		select {
		case b := <-h.broadCast:
			go func() {
				for name := range h.clients {
					h.clients[name].message <- &pim.BroadcastMessage{Message: b.Message}
				}
			}()
		case <-h.exit:
			return
		}
	}
}
