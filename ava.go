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
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/aiakit/ava/logger"
	"github.com/panjf2000/ants/v2"
	"github.com/rs/cors"
	"go.etcd.io/etcd/client/v3"
)

type Options func(option *Option)

type Option struct {
	//transportServer id can be set,or random
	//this is the unique identifier of the service
	Id string

	//transportServer name,eg.srv.hello or api.hello
	Name string

	//random port
	//Random ports make you don’t need to care about specific port numbers,
	//which are more commonly used in internal services
	RandPort *[2]int

	//socket tcp ip:port address
	TcpPort int

	//websocket ip:port address
	WssAddress string

	//websocket relative path address of websocket
	WssPath string

	//http service address
	HttpAddress string

	//buffSize to data tunnel if it's need
	BuffSize int

	//transportServer transport
	TransportServer *rsocketServer

	//error packet
	//It will affect the format of the data you return
	Err ErrorPackager

	//receive system signal
	Signal []os.Signal

	//wrapper some middleware
	//it's can be interrupt
	//just for request response
	Wrappers []WrapperHandler

	//just for http request before or socket setup before
	Dog []DogHandler

	Hijacker []HijackHandler

	//when transportServer exit,will do exit func
	Exit []func()

	//etcd config
	EtcdConfig *clientv3.Config

	//config options
	ConfigOpt []configOptions

	//service discover registry
	Registry *etcdRegistry

	//service discover endpoint
	Endpoint *endpoint

	//only need cors middleware on ava http api POST/DELETE/GET/PUT/OPTIONS method
	Cors *cors.Cors

	LocalIp string

	EndpointIp string

	//root router redirect
	HttpGetRootPath string
}

func NewOpts(opts ...Options) Option {
	opt := Option{}

	for i := range opts {
		opts[i](&opt)
	}

	if opt.EtcdConfig == nil {
		opt.EtcdConfig = &clientv3.Config{
			Endpoints:   []string{"127.0.0.1:2379"},
			DialTimeout: time.Second * 5,
		}
	}

	// init e.DefaultEtcd
	err := chaosEtcd(time.Second*5, 60, opt.EtcdConfig)
	if err != nil {
		panic("etcdConfig occur error: " + err.Error())
	}

	err = NewConfig(opt.ConfigOpt...)
	if err != nil {
		panic("config NewConfig occur error: " + err.Error())
	}

	opt.Registry = newRegistry()

	if opt.Name == "" {
		opt.Name = getProjectName()
	}

	if opt.Id == "" {
		//todo change to git commit id+timestamp
		opt.Id = newUUID()
	}

	ip, err := LocalIp()
	if err != nil {
		panic(err)
	}
	opt.LocalIp = ip

	if opt.RandPort == nil {
		opt.RandPort = &[2]int{10000, 59999}
	}

	// NOTICE: api service only support fixed tcpAddress ,not suggest rand tcpAddress in api service
	if opt.TcpPort == 0 {
		opt.TcpPort = RandInt(opt.RandPort[0], opt.RandPort[1])
	}

	if opt.WssAddress != "" && opt.WssPath == "" {
		opt.WssPath = "/ava/wss"
	}

	if opt.WssPath != "" {
		if !strings.HasPrefix(opt.WssPath, "/") {
			opt.WssPath = "/" + opt.WssPath
		}

		if strings.HasSuffix(opt.WssPath, "/") {
			opt.WssPath = strings.TrimSuffix(opt.WssPath, "/")
		}
	}

	if opt.Err == nil {
		opt.Err = defaultErrorPacket
	}

	if opt.BuffSize == 0 {
		opt.BuffSize = 10
	}

	tcpAddress := opt.LocalIp + ":" + strconv.Itoa(opt.TcpPort)

	if opt.TransportServer == nil {
		opt.TransportServer = newRsocketServer(tcpAddress, opt.WssAddress, opt.Name, opt.BuffSize)
	}

	var endpointIp = opt.LocalIp
	if opt.EndpointIp != "" {
		endpointIp = opt.EndpointIp
	}

	endpointAddress := endpointIp + ":" + strconv.Itoa(opt.TcpPort)
	if opt.Endpoint == nil {
		opt.Endpoint, err = newEndpoint(opt.Id, opt.Name, endpointAddress)
		if err != nil {
			panic(err)
		}
	}

	if opt.Signal == nil {
		opt.Signal = defaultSignal
	}

	if opt.Cors == nil {
		//allowed all
		opt.Cors = cors.AllowAll()
	}

	return opt
}

// getProjectName get current project name
func getProjectName() string {
	f := func(s string, pos int) string {
		runes := []rune(s)
		return string(runes[pos:])
	}
	// GetPwd get current file directory
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	directory := filepath.Dir(ex)

	return strings.Trim(f(directory, strings.LastIndex(directory, string(os.PathSeparator))), string(os.PathSeparator))
}

type Service struct {
	//service options setting
	opts Option

	//service exit channel
	exit chan struct{}

	//ava service client,for rpc to server
	client *Client

	//ava service server,listen and wait call
	server *Server
}

var defaultService *Service

func SetupService(opts ...Options) {
	defaultService = &Service{
		opts: NewOpts(opts...),
		exit: make(chan struct{}),
	}

	defaultService.server = newServer(defaultService.opts)
}

func AvaClient() *Client {
	if defaultService.client == nil {
		defaultService.client = newClient(defaultService.opts.Endpoint, defaultService.opts.Registry)
	}

	return defaultService.client

}

func AvaServer() *Server {
	return defaultService.server
}

func Run() {
	defer func() {
		if r := recover(); r != nil {
			Stack(r)
		}
	}()

	// handler signal
	ch := make(chan os.Signal)
	signal.Notify(ch, defaultService.opts.Signal...)

	go func() {
		select {
		case c := <-ch:

			Infof("received signal %s ,service [%s] exit!", strings.ToUpper(c.String()), defaultService.opts.Name)

			defaultService.CloseService()

			for _, f := range defaultService.opts.Exit {
				f()
			}

			defaultService.exit <- struct{}{}
		}
	}()

	defaultService.server.Run()

	select {
	case <-defaultService.exit:
	}

	os.Exit(0)
}

func (s *Service) CloseService() {
	//close registry service discover
	if s.opts.Registry != nil {
		_ = s.opts.Registry.Deregister(s.opts.Endpoint)
		s.opts.Registry.CloseRegistry()
		s.opts.Registry = nil
	}

	//close service client
	if s.client != nil {
		s.client.CloseClient()
	}

	//close service server
	if s.server != nil {
		s.server.CloseServer()
	}

	//close config setting
	configClose()

	defaultEtcd.CloseEtcd()

	//todo flush content
	logger.Close()
}

const SupportPackageIsVersion1 = 1

func BuffSize(buffSize int) Options {
	return func(o *Option) {
		o.BuffSize = buffSize
	}
}

func Wrapper(wrappers ...WrapperHandler) Options {
	return func(o *Option) {
		o.Wrappers = append(o.Wrappers, wrappers...)
	}
}

func WatchDog(wrappers ...DogHandler) Options {
	return func(o *Option) {
		o.Dog = wrappers
	}
}

func Hijacker(hijacker ...HijackHandler) Options {
	return func(o *Option) {
		o.Hijacker = hijacker
	}
}

func Exit(exit ...func()) Options {
	return func(o *Option) {
		o.Exit = exit
	}
}

func Signal(signal ...os.Signal) Options {
	return func(o *Option) {
		o.Signal = signal
	}
}

// Port port[0]:min port[1]:max
func Port(port [2]int) Options {
	return func(o *Option) {
		if port[0] > port[1] {
			panic("port[1] must greater than port[0]")
		}

		if port[0] < 10000 {
			panic("rand port for internal transportServer suggest more than 10000")
		}

		o.RandPort = &port
	}
}

func ErrorPack(err ErrorPackager) Options {
	return func(o *Option) {
		o.Err = err
	}
}

func WssApiAddr(address, path string) Options {
	return func(o *Option) {
		o.WssAddress = address
		o.WssPath = path
	}
}

func HttpApiAdd(address string) Options {
	return func(o *Option) {
		o.HttpAddress = address
	}
}

func EndpointIp(endpointIp string) Options {
	return func(o *Option) {
		o.EndpointIp = endpointIp
	}
}

func Id(id string) Options {
	return func(o *Option) {
		o.Id = id
	}
}

func Namespace(name string) Options {
	return func(o *Option) {
		o.Name = name
	}
}

// HttpGetRootPathRedirect only one get request is supported,
// eg. http://localhost:9999/1234.txt ---> /hello/hello/say
func HttpGetRootPathRedirect(path string) Options {
	return func(option *Option) {
		option.HttpGetRootPath = path
	}
}

// EtcdConfig setting global etcd config first
func EtcdConfig(e *clientv3.Config) Options {
	return func(o *Option) {
		o.EtcdConfig = e
	}
}

func TCPApiPort(port int) Options {
	return func(o *Option) {
		o.TcpPort = port
	}
}

func ConfigOption(opts ...configOptions) Options {
	return func(o *Option) {
		o.ConfigOpt = opts
	}
}

func AddCodec(contentType string, c Codecs) Options {
	return func(o *Option) {
		addCodec(contentType, c)
	}
}

func ServerVersion(version string) Options {
	return func(o *Option) {
		defaultVersion = version
	}
}

func ConnectTimeout(timeout time.Duration) Options {
	return func(o *Option) {
		defaultConnectTimeout = timeout
	}
}

func KeepaliveInterval(keepaliveInterval time.Duration) Options {
	return func(o *Option) {
		defaultKeepaliveInterval = keepaliveInterval
	}
}

func KeepaliveLifetime(keepaliveLifetime time.Duration) Options {
	return func(o *Option) {
		defaultKeepaliveLifetime = keepaliveLifetime
	}
}

func Cors(m *cors.Cors) Options {
	return func(option *Option) {
		option.Cors = m
	}
}

func LogLevel(level logger.Level) {
	logger.DefaultOutput.SetLevel(level)
}

var AntsPool, _ = ants.NewPool(runtime.NumCPU() * 10)
