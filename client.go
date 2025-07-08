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
	"github.com/gogo/protobuf/codec"
	"github.com/gogo/protobuf/proto"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Client to invoke to mockServer
type Client struct {

	//this endpoint for client to invoke mockServer
	e *endpoint

	registry *etcdRegistry

	//The strategy used by the client to initiate an rpc request, with roundRobin or direct ip request
	strategy Strategy
}

func newClient(e *endpoint, registry *etcdRegistry) *Client {
	s := &Client{e: e, registry: registry}

	s.strategy = newStrategy(e, registry)

	return s
}

// InvokeRR rpc request requestResponse,it's block request,one request one response
func (s *Client) InvokeRR(
	c *Context,
	method string,
	req, rsp proto.Message,
	opts ...InvokeOptions,
) error {

	// new a invoke setting
	cc, invoke, err := newInvoke(c, method, opts...)
	if err != nil {
		c.Error(err)
		return err
	}

	if invoke.Broadcast() {
		cons, err := s.strategy.GetAllScope(invoke.Scope())
		if err != nil {
			cc.Error(err)
			return err
		}

		for i := range cons {
			err = AntsPool.Submit(func() {
				//don't broadcast to yourself
				if cons[i].E.Id == s.e.Id {
					return
				}
				if invoke.FF() {
					invoke.InvokeFF(cc, req, cons[i])
					return
				}

				err := invoke.InvokeRR(cc, req, rsp, cons[i])
				if err != nil {
					err = errors.New(fmt.Sprintf("previous=%v |address=%s |err=%v", err, cons[i].E.Address, err))
					c.Errorf("address=%s |err=%v", cons[i].E.Address, err)
					return
				}
			})

			if err != nil {
				c.Error(err)
				return err
			}
		}

		return nil
	}

	var cnn *conn

	// if address is nil ,user roundRobin strategy
	// otherwise straight to newInvoke ip mockServer
	switch {
	case invoke.Address() != "":
		cnn, err = s.strategy.StraightByAddr(invoke.Scope(), invoke.Address())
	case invoke.Id() != "":
		cnn, err = s.strategy.StraightById(invoke.Scope(), invoke.Id())
	default:
		cnn, err = s.strategy.Next(invoke.Scope())
	}

	if err != nil {
		cc.Error(err)
		return err
	}

	if cnn.GetState() != stateWorking {
		return errors.New("con is not working")
	}

	if invoke.FF() {
		invoke.InvokeFF(cc, req, cnn)
		return nil
	}

	return invoke.InvokeRR(cc, req, rsp, cnn)
}

// InvokeRS rpc request requestStream,it's one request and multiple response
func (s *Client) InvokeRS(
	c *Context,
	method string,
	req proto.Message,
	opts ...InvokeOptions,
) chan []byte {

	// new a invoke setting
	cc, invoke, err := newInvoke(c, method, opts...)
	if err != nil {
		// create a chan error response
		c.Error(err)
		return nil
	}

	var cnn *conn

	// if address is nil ,user roundRobin strategy
	// otherwise straight to newInvoke ip mockServer
	switch {
	case invoke.Address() != "":
		cnn, err = s.strategy.StraightByAddr(invoke.Scope(), invoke.Address())
	case invoke.Id() != "":
		cnn, err = s.strategy.StraightById(invoke.Scope(), invoke.Id())
	default:
		cnn, err = s.strategy.Next(invoke.Scope())
	}

	if err != nil {
		cc.Error(err)
		return nil
	}

	//encode req body to ava packet
	b, err := c.Codec().Encode(req)

	if err != nil {
		// create a chan error response
		c.Error(err)
		return nil
	}

	return cnn.getRsocketClient().RS(cc, payload2avaPacket(b))
}

// InvokeRC rpc request requestChannel,it's multiple request and multiple response
func (s *Client) InvokeRC(
	c *Context,
	method string,
	req chan []byte,
	opts ...InvokeOptions,
) chan []byte {

	// new a newInvoke setting
	cc, invoke, err := newInvoke(c, method, opts...)
	if err != nil {
		c.Error(err)
		// create a chan error response
		return nil
	}

	var cnn *conn

	// if address is nil ,user roundRobin strategy
	// otherwise straight to newInvoke ip mockServer
	switch {
	case invoke.Address() != "":
		cnn, err = s.strategy.StraightByAddr(invoke.Scope(), invoke.Address())
	case invoke.Id() != "":
		cnn, err = s.strategy.StraightById(invoke.Scope(), invoke.Id())
	default:
		cnn, err = s.strategy.Next(invoke.Scope())
	}
	if err != nil {
		cc.Error(err)
		// create a chan error response
		return nil
	}

	return cnn.getRsocketClient().RC(cc, req)
}

func (s *Client) CloseClient() {
	if s.strategy != nil {
		s.strategy.CloseStrategy()
	}
}

type InvokeOptions func(*InvokeOption)

type InvokeOption struct {

	//scope is the service discovery prefix key
	scope string

	//address is witch mockServer you want to call
	address string

	id string

	//serviceName is witch mockServer by service serviceName
	serviceName string

	//version is witch mockServer by version
	version string

	//buffSize effective only requestChannel
	buffSize int

	trace string

	prefix string

	//for requestResponse try to retry request
	retry int

	//data encoding or decoding
	cc codec.Codec

	//FF
	ff bool

	//Broadcast to all other services except this server
	broadcast bool
}

func ClientCodec(cc codec.Codec) InvokeOptions {
	return func(option *InvokeOption) {
		option.cc = cc
	}
}

func FF() InvokeOptions {
	return func(option *InvokeOption) {
		option.ff = true
	}
}

func Broadcast() InvokeOptions {
	return func(option *InvokeOption) {
		option.broadcast = true
	}
}

// WithTracing set tracing
func WithTracing(t string) InvokeOptions {
	return func(invokeOption *InvokeOption) {
		invokeOption.trace = t
	}
}

// InvokeBuffSize set buff size for requestChannel
func InvokeBuffSize(buffSize int) InvokeOptions {
	return func(invokeOption *InvokeOption) {
		invokeOption.buffSize = buffSize
	}
}

// WithName set service discover prefix with service serviceName
func WithName(name string, version ...string) InvokeOptions {
	return func(invokeOption *InvokeOption) {
		var ver = defaultVersion

		// if no version ,use default version number
		if len(version) == 1 {
			ver = version[0]
		}

		invokeOption.scope = name + "/" + ver
		invokeOption.serviceName = name
		invokeOption.version = ver

		invokeOption.prefix = name

		ss := strings.Split(invokeOption.prefix, ".")
		invokeOption.prefix = ss[len(ss)-1]

		if strings.HasSuffix(invokeOption.prefix, "/") {
			invokeOption.prefix = strings.TrimSuffix(invokeOption.prefix, "/")
		}

		if !strings.HasPrefix(invokeOption.prefix, "/") {
			invokeOption.prefix = "/" + invokeOption.prefix
		}
	}
}

// WithAddress set service discover prefix with both service serviceName and address
func WithAddress(name, address string, version ...string) InvokeOptions {
	return func(invokeOption *InvokeOption) {
		var ver = defaultVersion

		// if no version ,use default version number
		if len(version) == 1 {
			ver = version[0]
		}

		invokeOption.scope = name + "/" + ver
		invokeOption.address = address
		invokeOption.serviceName = name
		invokeOption.version = ver
	}
}

func WithId(name, id string, version ...string) InvokeOptions {
	return func(invokeOption *InvokeOption) {
		var ver = defaultVersion

		// if no version ,use default version number
		if len(version) == 1 {
			ver = version[0]
		}

		invokeOption.scope = name + "/" + ver
		invokeOption.id = id
		invokeOption.serviceName = name
		invokeOption.version = ver
	}
}

type Invoke struct {
	// invoke options
	opts InvokeOption
}

// newInvoke create a invoke
func newInvoke(c *Context, method string, opts ...InvokeOptions) (*Context, *Invoke, error) {
	invoke := &Invoke{}

	for i := range opts {
		opts[i](&invoke.opts)
	}

	if invoke.opts.serviceName == "" || invoke.opts.scope == "" {
		return nil, nil, errors.New("not set rpc service name")
	}

	method = invoke.opts.prefix + method

	// initialize tunnel for requestChannel only
	if invoke.opts.buffSize == 0 {
		invoke.opts.buffSize = 10
	}

	var meta = make(map[string]string, 3)
	if invoke.opts.version != "" {
		meta[defaultHeaderVersion] = invoke.opts.version
	}
	if invoke.opts.address != "" {
		meta[defaultHeaderAddress] = invoke.opts.address
	}

	meta[defaultHeaderContentType] = c.ContentType

	// clone context metadata
	cc, err := c.WithMetadata(invoke.opts.serviceName, method, meta)
	return cc, invoke, err
}

func (invoke *Invoke) Opts() InvokeOption {
	return invoke.opts
}

func (invoke *Invoke) Address() string {
	return invoke.opts.address
}

func (invoke *Invoke) Id() string {
	return invoke.opts.id
}

func (invoke *Invoke) Scope() string {
	return invoke.opts.scope
}

func (invoke *Invoke) FF() bool {
	return invoke.opts.ff
}

func (invoke *Invoke) Broadcast() bool {
	return invoke.opts.broadcast
}

// InvokeRR invokeRR is invokeRequestResponse
func (invoke *Invoke) InvokeRR(c *Context, req, rsp proto.Message, cnn *conn) error {
	// encoding req body to ava packet
	b, err := c.Codec().Encode(req)
	if err != nil {
		c.Error(err)
		return err
	}
	var request, response = payload2avaPacket(b), newPacket()

	err = invokeRR(c, cnn, invoke, request, response, rsp)

	Recycle(request)
	Recycle(response)

	recycleContext(c)

	return err
}

func invokeRR(
	c *Context,
	cnn *conn,
	invoke *Invoke,
	request, response *Packet,
	rsp proto.Message,
) error {

	// send a request by requestResponse
	err := cnn.getRsocketClient().RR(c, request, response)
	if err != nil {
		if invoke.opts.retry > 0 {
			// to retry request with backoff
			bf := backoffInstance()
			for i := 0; i < invoke.opts.retry; i++ {
				time.Sleep(bf.Next(i))
				if err = cnn.getRsocketClient().RR(c, request, response); err == nil {
					break
				}
			}

			if err != nil {
				c.Error(err)

				// mark error count to manager conn state
				cnn.GrowError()
				return err
			}
		}
		return err
	}

	return c.Codec().Decode(response.Bytes(), rsp)
}

// InvokeFF invokeFF is FireAndForget
func (invoke *Invoke) InvokeFF(c *Context, req proto.Message, cnn *conn) {
	// encoding req body to ava packet
	b, err := c.Codec().Encode(req)
	if err != nil {
		c.Error(err)
		return
	}
	var request = payload2avaPacket(b)

	// send a request by FireAndForget
	cnn.getRsocketClient().FF(c, request)

	// defer Recycle packet to pool
	Recycle(request)

	recycleContext(c)
}

var (
	errorNoneServer   = errors.New("server is none to use")
	errorNoSuchServer = errors.New("no such server")
)

type Strategy interface {

	//Next Round-robin scheduling
	Next(scope string) (next *conn, err error)

	//StraightByAddr direct call
	StraightByAddr(scope, address string) (next *conn, err error)

	//StraightById direct call
	StraightById(scope, id string) (next *conn, err error)

	//CloseStrategy Strategy
	CloseStrategy()

	// GetAllScope get all scope services
	GetAllScope(scope string) ([]*conn, error)
}

var _ Strategy = &strategy{}

type strategy struct {
	sync.Mutex

	//per service & multiple conn
	connPerService map[string]*pod

	//discover registry
	registry *etcdRegistry

	//registry update callback action
	action chan *registryReceive

	//close strategy signal
	close chan struct{}

	localEndpoint *endpoint
}

// newStrategy create a strategy
func newStrategy(
	local *endpoint,
	registry *etcdRegistry,
) Strategy {
	s := &strategy{
		connPerService: make(map[string]*pod),
		registry:       registry,
		close:          make(chan struct{}),
		localEndpoint:  local,
	}

	//receive registry update notify
	s.action = s.registry.Watch()

	//Synchronize all existing services
	//s.lazySync()

	//handler registry notify
	go s.notify()

	return s
}

// get a pod,if is nil ,create a new pod
func (s *strategy) getOrSet(scope string) (*pod, error) {
	p, ok := s.connPerService[scope]
	if !ok || p.count == 0 {
		s.Lock()
		defer s.Unlock()

		e, err := s.registry.List()
		if err != nil {
			return nil, err
		}

		for i := range e {
			if e[i].Scope == scope {
				err = s.sync(e[i])
				if err != nil {
					return nil, err
				}
			}
		}
		v, ok := s.connPerService[scope]
		if !ok {
			return nil, fmt.Errorf("no such scope node service [%s]", scope)
		}
		return v, nil
	}
	//pod must available
	return p, nil
}

// Next Round-robin next
func (s *strategy) Next(scope string) (*conn, error) {
	p, err := s.getOrSet(scope)
	if err != nil {
		return nil, err
	}

	var c *conn
	for i := 0; i < p.count; i++ {
		c = p.clients[(int(atomic.AddUint32(&p.index, 1))-1)%p.count]
		if c.Running() {
			break
		}
	}

	if c == nil || !c.Running() {
		return nil, errorNoneServer
	}

	return c, nil
}

func (s *strategy) GetAllScope(scope string) ([]*conn, error) {
	p, err := s.getOrSet(scope)
	if err != nil {
		return nil, err
	}
	return p.clients, nil
}

// StraightByAddr direct invoke
func (s *strategy) StraightByAddr(scope, address string) (*conn, error) {
	p, err := s.getOrSet(scope)
	if err != nil {
		return nil, err
	}

	c, ok := p.clientsMap[address]
	if !ok || !c.Running() {
		return nil, errorNoSuchServer
	}

	return c, nil
}

func (s *strategy) StraightById(scope, id string) (*conn, error) {
	p, err := s.getOrSet(scope)
	if err != nil {
		return nil, err
	}

	c, ok := p.clientsMapId[id]
	if !ok || !c.Running() {
		return nil, errorNoSuchServer
	}

	return c, nil
}

// Synchronize all existing services
func (s *strategy) lazySync() {
	s.Lock()
	defer s.Unlock()

	es, err := s.registry.List()
	if err != nil {
		Error(err)
		return
	}

	for _, e := range es {
		_ = s.sync(e)
	}
}

// Synchronize one services
func (s *strategy) sync(e *endpoint) error {

	//filter local service registry
	if e.Name == s.localEndpoint.Name {
		return nil
	}

	//if reflect.DeepEqual(e, s.localEndpoint) {
	//	return nil
	//}

	p, ok := s.connPerService[e.Scope]
	if !ok {
		p = newPod()
	}

	err := p.Add(e)
	if err != nil {
		Error(err)
		return err
	}

	s.connPerService[e.Scope] = p

	return nil
}

// receive a registry notify callback
func (s *strategy) notify() {

QUIT:
	for {
		select {
		case act := <-s.action:
			Debug("update endpoint was changed", MustMarshalString(act))

			s.update(act)
		case <-s.close:
			break QUIT
		}
	}
}

// update a registry notify
func (s *strategy) update(act *registryReceive) {
	s.Lock()
	defer s.Unlock()

	switch act.Act {
	case watcherCreate, watcherUpdate:
		_ = s.sync(act.E)
	case watcherDelete:
		p, ok := s.connPerService[act.E.Scope]
		if ok {
			p.Del(act.E.Address)
		}
	}
}

// CloseStrategy strategy close
func (s *strategy) CloseStrategy() {
	for _, p := range s.connPerService {
		for _, client := range p.clientsMap {
			client.CloseConn()
		}
	}

	s.close <- struct{}{}
}

type pod struct {
	sync.Mutex

	//serviceName
	serviceName string

	//count the all clients in this pod
	count int

	//Round-robin call cursor
	index uint32

	//clients array in pod
	clients []*conn

	//clientMap in pod
	clientsMap map[string]*conn

	clientsMapId map[string]*conn

	//when client occur a error,handler callback
	//callback is the mockServer address
	callback chan string
}

// create a pod
func newPod() *pod {
	return &pod{
		clients:      make([]*conn, 0, 10),
		clientsMap:   make(map[string]*conn),
		clientsMapId: make(map[string]*conn),
		callback:     make(chan string),
	}
}

// Add a client endpoint to pod
func (p *pod) Add(e *endpoint) error {

	p.Lock()
	defer p.Unlock()

	c, err := newConn(e, p.callback)
	if err != nil {
		return err
	}

	c.E = e

	//setting conn array cursor
	c.SetCursor(len(p.clients))

	p.count += 1
	p.serviceName = e.Name
	p.clients = append(p.clients, c)
	p.clientsMap[e.Address] = c
	p.clientsMapId[e.Id] = c

	// update callback
	go p.watch()

	// let client's conn working
	c.Working()

	return nil
}

// Del delete a client endpoint from pod
func (p *pod) Del(addr string) {
	p.Lock()
	defer p.Unlock()

	c, ok := p.clientsMap[addr]
	if ok {
		c.CloseConn()
		p.clients = append(p.clients[:c.Cursor()], p.clients[c.Cursor()+1:]...)
		delete(p.clientsMap, addr)
		delete(p.clientsMapId, c.E.Id)
		p.count -= 1
		p.index -= 1
	}
}

// watch callback mockServer address to delete
func (p *pod) watch() {
	for {
		select {
		case address := <-p.callback:
			p.Del(address)
		}
	}
}

type WatcherAction string

const (
	watcherCreate WatcherAction = "create"
	watcherUpdate               = "update"
	watcherDelete               = "delete"
)

// registryReceive w data change content
type registryReceive struct {
	Act WatcherAction

	E *endpoint

	//etcd key
	Key string
}

var (
	// defaultConnectTimeout dial rpc connection timeout
	// connect mockServer within connectTimeout
	// if out of ranges,will be timeout
	defaultConnectTimeout = time.Second * 5
	// defaultKeepaliveInterval rpc keepalive interval time
	//keepalive setting,the period for requesting heartbeat to stay connected
	defaultKeepaliveInterval = time.Second * 5
	// defaultKeepaliveLifetime rpc keepalive lifetime
	//keepalive setting,the longest time the connection can survive
	defaultKeepaliveLifetime = time.Second * 600
)

// state is mark conn state,conn must safe
type state = uint32

const (
	// stateBlock block is unavailable state
	stateBlock state = 0x01 + iota
	// stateReady ready is unavailable state
	stateReady

	// stateWorking is available state
	stateWorking

	// errCountDelta is record the number of connection failures
	errCountDelta = 3
)

// conn include transport client
type conn struct {
	sync.Mutex

	cursor int

	// conn state
	state state

	// current conn occur error count
	errCount uint32

	// client per conn contain rsocketClient
	client *rsocketClient

	E *endpoint
}

func (c *conn) SetCursor(i int) {
	c.cursor = i
}

func (c *conn) Cursor() int {
	return c.cursor
}

// GrowErrorCount error safe grow one
func (c *conn) GrowErrorCount() uint32 {
	return atomic.AddUint32(&c.errCount, 1)
}

// Working swap state to working
func (c *conn) Working() {
	atomic.SwapUint32(&c.state, stateWorking)
}

// Block swap state to block
func (c *conn) Block() {
	atomic.SwapUint32(&c.state, stateBlock)
}

// Ready swap state to ready
func (c *conn) Ready() {
	atomic.SwapUint32(&c.state, stateReady)
}

// GetState get state
func (c *conn) GetState() state {
	return atomic.LoadUint32(&c.state)
}

// Running judge state is working
func (c *conn) Running() bool {
	return c.GetState() == stateWorking
}

// IsBlock judge state is block
func (c *conn) IsBlock() bool {
	return c.GetState() == stateBlock
}

// GrowError grow error and let the error conn retry working util conn is out of serviceName
func (c *conn) GrowError() {
	c.Lock()
	defer c.Unlock()

	if c.GrowErrorCount() > errCountDelta && c.Running() {
		// let conn block
		c.Block()
		go func() {
			select {
			case <-time.After(time.Second * 3):
				// let conn working
				// if conn is out of serviceName,this is not effect
				//todo try to ping ,if ok let client working
				//if close ,don't do anything
				c.Working()
			}
		}()
	}
}

// newConn is create a conn
// closeCallBack is the conn client occur error and callback
func newConn(
	e *endpoint,
	closeCallback chan string,
) (*conn, error) {
	client := newRsocketClient(
		defaultConnectTimeout,
		defaultKeepaliveInterval,
		defaultKeepaliveLifetime,
	)
	err := client.Dial(e, closeCallback)
	if err != nil {
		return nil, err
	}

	c := &conn{client: client}

	// change state to ready
	c.Ready()

	return c, nil
}

// Client get client
func (c *conn) getRsocketClient() *rsocketClient {
	return c.client
}

// CloseConn close client connection
func (c *conn) CloseConn() {
	c.Block()

	if c.client != nil {
		c.getRsocketClient().CloseClient()
		c.client = nil
	}
}

// Truncated Binary Exponential Back—off,TBEB
// After the collision site stops sending, it will not send data again immediately,
// but back off for a random time,
// reducing the probability of collision during retransmission.
const defaultFactor = 2

var defaultBackoff = newBackoff()

type backOff struct {
	factor, delayMin, delayMax float64
	Attempts                   int
}

func newBackoff() *backOff {
	return &backOff{
		factor:   defaultFactor,
		delayMin: 10,
		delayMax: 1000,
	}
}

func backoffInstance() *backOff {
	return defaultBackoff.clone()
}

func WitchBackoff(factor, delayMin, delayMax float64, attempts int) *backOff {
	b := defaultBackoff.clone()
	b.factor = factor
	b.delayMax = delayMax
	b.delayMin = delayMin
	b.Attempts = attempts
	return b
}

// Next
// Exponential
func (b *backOff) Next(delta int) time.Duration {
	r := b.delayMin * math.Pow(b.factor, float64(b.Attempts))
	b.Attempts += delta
	if r > b.delayMax {
		return b.duration(b.delayMax)
	}

	if r < b.delayMin {
		return b.duration(b.delayMin)
	}

	return b.duration(r)
}

func (b *backOff) duration(t float64) time.Duration {
	return time.Millisecond * time.Duration(t)
}

func (b *backOff) clone() *backOff {
	cb := *b
	return &cb
}
