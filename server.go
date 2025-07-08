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
	ctx "context"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

var defaultSignal = []os.Signal{
	syscall.SIGINT,
	syscall.SIGHUP,
	syscall.SIGTERM,
	syscall.SIGKILL,
	syscall.SIGQUIT,
}

var (
	defaultVersion = "v1.0.0"
	defaultSchema  = "goava"
)

type header = string

const (
	defaultHeaderVersion     header = "X-Api-Version"
	defaultHeaderTrace       header = "X-Api-Trace"
	defaultHeaderAddress            = "X-Api-Address"
	defaultHeaderSetup              = "X-Api-Setup"
	defaultHeaderContentType        = "Content-Type"
)

func ExitSignal(signal ...os.Signal) []os.Signal {
	return signal
}

type endpoint struct {
	//endpoint unique id
	Id string

	//endpoint name
	Name string

	//endpoint version
	Version string

	// schema/name/version/id
	Absolute string

	//service server ip address
	Address string

	// name.version
	// eg. api.hello/v.1.0.0
	Scope string
}

// newEndpoint new endpoint with schema,id,name,version,address
func newEndpoint(id, name, address string) (*endpoint, error) {
	if name == "" || address == "" || id == "" {
		return nil, errors.New("not complete")
	}
	e := new(endpoint)
	e.Id = id
	e.Name = name
	e.Version = defaultVersion
	e.Address = address
	e.Scope = e.Name + "/" + e.Version
	e.Absolute = defaultSchema + "/" + e.Scope + "/" + e.Id
	return e, nil
}

func (e *endpoint) String() string {
	return e.Name + " |" + e.Id + " |" + e.Address
}

type Server struct {

	//wait for server init
	wg *sync.WaitGroup

	//run transportServer option
	opts Option

	//transportServer exit channel
	exit chan struct{}

	//rpc transportServer router collection
	route *Router

	//api http server
	httpServer *http.Server
}

func (s *Server) Id() string {
	return s.opts.Id
}

func (s *Server) Name() string {
	name := s.opts.Name
	ss := strings.Split(name, ".")

	if len(ss) > 1 {
		name = ss[len(ss)-1]
	}

	return name
}

func newServer(opts Option) *Server {
	s := &Server{
		wg:   new(sync.WaitGroup),
		opts: opts,
		exit: make(chan struct{}),
	}

	s.route = NewRouter(s.opts.Wrappers, s.opts.Err)

	s.opts.TransportServer.Accept(s.route)

	return s
}

func (s *Server) Run() {
	// echo method list
	s.route.List()

	s.opts.TransportServer.Run(s.wg)

	//run http transportServer
	if s.opts.HttpAddress != "" {
		go func() {

			//todo prefix
			prefix := s.Name()

			if !strings.HasPrefix(prefix, "/") {
				prefix = "/" + prefix
			}

			s.httpServer = &http.Server{
				Handler:      s.opts.Cors.Handler(s),
				Addr:         s.opts.HttpAddress,
				WriteTimeout: 300 * time.Second,
				ReadTimeout:  300 * time.Second,
				IdleTimeout:  time.Second * 200,
			}

			if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				Errorf("service %s |err=%v", s.opts.Name, err)
			}
		}()
	}

	s.wg.Wait()

	time.Sleep(time.Millisecond * 100)

	Infof(
		"[TCP:%s:%d][WS:%s][HTTP:%s] start success!",
		s.opts.LocalIp, s.opts.TcpPort,
		s.opts.WssAddress,
		s.opts.HttpAddress,
	)
	err := s.register()
	if err != nil {
		panic(err)
	}
}

func (s *Server) register() error {
	return s.opts.Registry.Register(s.opts.Endpoint)
}

func (s *Server) RegisterHandler(method string, rr Handler) {
	s.route.RegisterHandler(method, rr)
}

func (s *Server) RegisterStreamHandler(method string, rs StreamHandler) {
	s.route.RegisterStreamHandler(method, rs)
}

func (s *Server) RegisterChannelHandler(method string, rs ChannelHandler) {
	s.route.RegisterChannelHandler(method, rs)
}

func (s *Server) RegisterChannelMajorHandler(method string, rs ChannelHandler) {
	s.route.RegisterChannelHandler(method, rs)
}

// ava don't suggest method like GET,because you can use other http web framework
// to build a restful api with not by ava
// ava support POST,DELETE,PUT,GET,OPTIONS for compatible rrRouter ,witch request response way
// because ServeHTTP api need support json or proto data protocol
// suggest just use POST,PUT for your ava service
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var c = getContext()

	handlerServerHttp(c, s, w, r)

	recycleContext(c)
}

func (s *Server) CloseServer() {
	cc, cancel := ctx.WithTimeout(ctx.Background(), time.Second*5)
	defer cancel()

	if s.httpServer != nil {
		_ = s.httpServer.Shutdown(cc)
	}

	if s.opts.TransportServer != nil {
		s.opts.TransportServer.Close()
	}
}
