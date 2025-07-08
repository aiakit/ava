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

package ava

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/gogo/protobuf/proto"
)

// Handler for rpc service handler
type Handler func(c *Context, req *Packet, interrupt Interceptor) (rsp proto.Message, err error)

// StreamHandler for rpc service stream handler
type StreamHandler func(c *Context, req *Packet, exit chan struct{}) chan proto.Message

// ChannelHandler for rpc service channel handler
type ChannelHandler func(c *Context, req chan *Packet, exit chan struct{}) chan proto.Message

// Fire run interceptor action
type Fire func(c *Context, req proto.Message) proto.Message

// Interceptor for rpc request response interceptor function
type Interceptor func(c *Context, req proto.Message, fire Fire) (proto.Message, error)

// WrapperHandler for all rpc function middleware
type WrapperHandler func(c *Context) (proto.Message, error)

// DogHandler is before socket establish connection to check
type DogHandler func(c *Context) (proto.Message, error)

// HijackHandler is handle the interception of http requests
type HijackHandler func(c *Context, r *http.Request, w http.ResponseWriter, req, rsp *Packet) (abort bool)

var (
	errNotFoundHandler = "not found route path [%s]"
	errAbort           = errors.New("some hijacker  abort")
)

type Router struct {
	sync.Mutex
	//requestResponse map cache handler
	rrRoute map[string]Handler

	//requestStream map cache streamHandler
	rsRoute map[string]StreamHandler

	//requestChannel map cache channelHandler
	rcRoute map[string]ChannelHandler

	//wrapper middleware
	wrappers []WrapperHandler

	//configurable error message return
	errorPacket ErrorPackager
}

// NewRouter create a new Router
func NewRouter(wrappers []WrapperHandler, err ErrorPackager) *Router {
	return &Router{
		rrRoute:     make(map[string]Handler),
		rsRoute:     make(map[string]StreamHandler),
		rcRoute:     make(map[string]ChannelHandler),
		wrappers:    wrappers,
		errorPacket: err,
	}
}

func (r *Router) Error() ErrorPackager {
	return r.errorPacket
}

func (r *Router) RegisterHandler(method string, rr Handler) {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.rrRoute[method]; ok {
		panic("this rrRoute is already exist:" + method)
	}
	r.rrRoute[method] = rr
}

func (r *Router) RegisterStreamHandler(method string, rs StreamHandler) {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.rsRoute[method]; ok {
		panic("this rsRoute is already exist:" + method)
	}

	r.rsRoute[method] = rs
}

func (r *Router) RegisterChannelHandler(service string, rc ChannelHandler) {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.rcRoute[service]; ok {
		panic("this rcRoute is already exist:" + service)
	}

	r.rcRoute[service] = rc
}

func (r *Router) FF(c *Context, req *Packet) error {
	rrHandler, ok := r.rrRoute[c.Metadata.Method()]
	if !ok {
		return fmt.Errorf(errNotFoundHandler, c.Metadata.Method())
	}

	_, err := rrHandler(c, req, r.ffInterrupt())
	return err
}

func (r *Router) RR(c *Context, req *Packet, rsp *Packet) error {
	rrHandler, ok := r.rrRoute[c.Metadata.Method()]
	if !ok {
		return fmt.Errorf(errNotFoundHandler, c.Metadata.Method())
	}

	resp, err := rrHandler(c, req, r.rrInterrupt())
	if resp != nil {
		b, err := c.Codec().Encode(resp)
		if err != nil {
			c.Error(err)
			return err
		}

		rsp.Write(b)
	}

	return err
}

func (r *Router) RS(c *Context, req *Packet, exit chan struct{}) (chan proto.Message, error) {

	// rrInterrupt
	for i := range r.wrappers {
		_, err := r.wrappers[i](c)
		if err != nil {
			c.Errorf("wrappers err=%v", err)
			return nil, err
		}
	}

	rsHandler, ok := r.rsRoute[c.Metadata.Method()]
	if !ok {
		return nil, fmt.Errorf(errNotFoundHandler, c.Metadata.Method())
	}

	return rsHandler(c, req, exit), nil
}

func (r *Router) RC(c *Context, req chan *Packet, exit chan struct{}) (chan proto.Message, error) {
	// rrInterrupt when occur error
	for i := range r.wrappers {
		_, err := r.wrappers[i](c)
		if err != nil {
			c.Errorf("wrappers err=%v", err)
			return nil, err
		}
	}

	rcHandler, ok := r.rcRoute[c.Metadata.Method()]
	if !ok {
		return nil, fmt.Errorf(errNotFoundHandler, c.Metadata.Method())
	}

	return rcHandler(c, req, exit), nil
}

func (r *Router) rrInterrupt() Interceptor {
	return func(c *Context, req proto.Message, fire Fire) (proto.Message, error) {
		// rrInterrupt when occur error
		for i := range r.wrappers {
			rsp, err := r.wrappers[i](c)
			if err != nil {
				c.Errorf("wrappers err=%v", err)
				return rsp, err
			}
		}

		rsp := fire(c, req)

		reqData := c.Codec().MustEncodeString(req)
		rspData := c.Codec().MustEncodeString(rsp)

		if !c.isPutFile && len(reqData) < 1024 && len(rspData) < 1024 {
			c.Debugf("FROM=%s |TO=%s |PATH=%s", reqData, rspData, c.Metadata.Method())
		} else {
			c.Debugf("reqData_len=%d |rspData_len=%d |PATH=%s", len(reqData), len(rspData), c.Metadata.Method())
		}
		return rsp, nil
	}
}

func (r *Router) ffInterrupt() Interceptor {
	return func(c *Context, req proto.Message, fire Fire) (proto.Message, error) {
		// rrInterrupt when occur error
		for i := range r.wrappers {
			rsp, err := r.wrappers[i](c)
			if err != nil {
				c.Errorf("wrappers err=%v", err)
				return rsp, err
			}
		}

		fire(c, req)

		reqData := c.Codec().MustEncodeString(req)

		if !c.isPutFile && len(reqData) < 10<<10 {
			c.Debugf("FROM=%s |PATH=%s", reqData, c.Metadata.Method())
		}
		return nil, nil
	}
}

func (r *Router) List() {
	Info("REGISTERED ROUTER LIST:")
	for k := range r.rrRoute {
		Infof("☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵ RR ☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵ [%s]", k)
	}

	for k := range r.rsRoute {
		Infof("☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵ RS ☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵ [%s]", k)
	}

	for k := range r.rcRoute {
		Infof("☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵ RC ☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵☵ [%s]", k)
	}
}
