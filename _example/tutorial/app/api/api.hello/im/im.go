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
	"fmt"
	"github.com/aiakit/ava"
	"github.com/aiakit/ava/_example/tutorial/proto/pim"
)

type Im struct {
	h *hub
}

func NewIm() *Im {
	return &Im{h: defaultHub}
}

// Broadcast receive broadcast message from other service
func (i *Im) Broadcast(c *ava.Context, req *pim.BroadcastMessage, rsp *pim.BroadcastMessage) {
	//broadcast to
	i.h.broadCast <- &pim.BroadcastMessage{
		Message: req.Message,
	}
}

func (i *Im) Send(c *ava.Context, req *pim.SayReq, rsp *pim.SayRsp) {
	if i.h.clients[req.Name] == nil {
		c.Error("handshake first......", req.Name)
		return
	}

	c.Infof("name=%s |message=%s", req.Name, req.Message)

	if _, ok := i.h.clients[req.Name]; !ok {
		return
	}

	//broadcast to service im/broadcast function,not including himself,the local service 'Send'
	//and return directly to the current service
	//ipc.Broadcast(c, &pim.BroadcastMessage{Message: req.SceneName + ": " + req.Message})
	//rsp.Message = req.SceneName + ": " + req.Message

	//local service response
	i.h.broadCast <- &pim.BroadcastMessage{
		Message: req.Name + ": " + req.Message,
	}
}

func (i *Im) Handshake(c *ava.Context, req *pim.HandshakeReq, exit chan struct{}) chan *pim.BroadcastMessage {
	if req.Name == "" {
		c.Errorf("handshake name is nil", req.Name)
	}

	//add client to hub
	var rsp = make(chan *pim.BroadcastMessage, 100)

	p := &point{name: req.Name, message: rsp}
	i.h.registerClient(p)

	go func() {

		<-exit
		fmt.Println("-----exit------", req.Name)
		i.h.unregisterClient(p)

		close(rsp)
	}()

	c.Debugf("handshake... |name=%s", req.Name)
	return rsp
}
