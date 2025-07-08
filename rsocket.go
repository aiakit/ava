package ava

import (
	ctx "context"
	"sync"
	"time"

	"github.com/jjeffcaii/reactor-go/scheduler"
	"github.com/rsocket/rsocket-go"
	"github.com/rsocket/rsocket-go/payload"
	"github.com/rsocket/rsocket-go/rx"
	"github.com/rsocket/rsocket-go/rx/flux"
	"github.com/rsocket/rsocket-go/rx/mono"
)

type rsocketClient struct {

	//rsocket client
	client rsocket.Client

	//rsocket connect timeout
	connectTimeout time.Duration

	//rsocket keepalive interval
	keepaliveInterval time.Duration

	//rsocket keepalive life time
	keepaliveLifetime time.Duration
}

func newRsocketClient(connTimeout, interval, tll time.Duration) *rsocketClient {
	return &rsocketClient{
		connectTimeout:    connTimeout,
		keepaliveInterval: interval,
		keepaliveLifetime: tll,
	}
}

func (cli *rsocketClient) Dial(e *endpoint, ch chan string) (err error) {
	cli.client, err = rsocket.
		Connect().
		//MetadataMimeType(extension.ApplicationProtobuf.String()).
		//DataMimeType(extension.ApplicationProtobuf.String()).
		//Scheduler(
		//	scheduler.NewElastic(runtime.NumCPU()<<8),
		//	scheduler.NewElastic(runtime.NumCPU()<<8),
		//). //set scheduler to best
		KeepAlive(cli.keepaliveInterval, cli.keepaliveLifetime, 3).
		ConnectTimeout(cli.connectTimeout).
		OnConnect(
			func(client rsocket.Client, err error) { //handler when connect success
				Debugf("connected at: %s", e.Address)
			},
		).
		OnClose(
			func(err error) { //when net occur some error,it's will be callback the error rsocketServer ip address
				if err != nil {
					Errorf("rsocketServer [%s] is closed |err=%v", e.String(), err)
				} else {
					Debugf("rsocketServer [%s] is closed", e.String())
				}

				ch <- e.Address
			},
		).
		Transport(rsocket.TCPClient().SetAddr(e.Address).Build()). //setup transport and start
		Start(ctx.TODO())
	return err
}

// RR requestResponse on blockUnsafe
func (cli *rsocketClient) RR(c *Context, req *Packet, rsp *Packet) (err error) {
	pl, release, err := cli.
		client.
		RequestResponse(payload.New(req.Bytes(), c.Metadata.Payload())).
		BlockUnsafe(ctx.Background())

	if err != nil {
		c.Error("socket err occurred ", err)
		return err
	}

	rsp.Write(pl.Data())

	release()

	return nil
}

func (cli *rsocketClient) FF(c *Context, req *Packet) {
	cli.client.FireAndForget(payload.New(req.Bytes(), c.Metadata.Payload()))
}

// RS requestStream
func (cli *rsocketClient) RS(c *Context, req *Packet) chan []byte {
	var (
		f   = cli.client.RequestStream(payload.New(req.Bytes(), c.Metadata.Payload()))
		rsp = make(chan []byte, 20)
	)

	f.
		SubscribeOn(scheduler.Parallel()).
		DoFinally(
			func(s rx.SignalType) {
				close(rsp)
				Recycle(req)
				recycleContext(c)
			},
		).DoOnError(
		func(e error) {
			c.Error(e)
		},
	).
		Subscribe(
			ctx.Background(),
			rx.OnNext(
				func(p payload.Payload) error {
					rsp <- payload.Clone(p).Data()
					return nil
				},
			),
			rx.OnError(
				func(err error) {
					c.Error(err)
				},
			),
		)

	return rsp
}

// RC requestChannel
func (cli *rsocketClient) RC(c *Context, req chan []byte) chan []byte {
	var (
		sendPayload = make(chan payload.Payload, cap(req))
	)

	go func() {
		sendPayload <- payload.New(c.Metadata.Payload(), nil)
	QUIT:
		for {
			select {
			case d, ok := <-req:
				if ok {
					pl := payload.New(d, nil)
					sendPayload <- pl
				} else {
					close(sendPayload)
					break QUIT
				}
			}
		}

	}()

	var (
		f = cli.client.RequestChannel(
			flux.Create(
				func(ctx ctx.Context, s flux.Sink) {
				loop:
					for {
						select {
						case <-ctx.Done():
							s.Error(ctx.Err())
							break loop
						case p, ok := <-sendPayload:
							if ok {
								s.Next(p)
							} else {
								s.Complete()
								break loop
							}
						}
					}
				},
			),
		)
		rsp = make(chan []byte, cap(req))
	)

	f.
		SubscribeOn(scheduler.Parallel()).
		DoFinally(
			func(s rx.SignalType) {
				//todo handler rx.SignalType
				close(rsp)
				recycleContext(c)
			},
		).
		Subscribe(
			ctx.Background(),
			rx.OnNext(
				func(p payload.Payload) error {
					rsp <- payload.Clone(p).Data()
					return nil
				},
			),
			rx.OnError(
				func(err error) {
					c.Debug(err)
				},
			),
		)

	return rsp
}

func (cli *rsocketClient) String() string {
	return "rsocket"
}

func (cli *rsocketClient) CloseClient() {
	if cli.client != nil {

		//todo here must go func
		go cli.client.Close()
		//cli.client = nil
	}
}

type rsocketServer struct {

	//wait rsocketServer run success
	wg *sync.WaitGroup

	//given serverName to service discovery to find
	serverName string

	//tcp socket address
	tcpAddress string

	//websocket address
	wssAddress string

	//requestChannel buffSize setting
	buffSize int

	//rsocket serverBuilder
	serverBuilder rsocket.ServerBuilder

	//rsocket serverStarter
	serverStart rsocket.ToServerStarter

	dog []DogHandler
}

func (r *rsocketServer) Address() string {
	return "[tcp: " + r.tcpAddress + "] [wss: " + r.wssAddress + "]"
}

func (r *rsocketServer) String() string {
	return "rsocket"
}

func newRsocketServer(tcpAddress, wssAddress, serverName string, buffSize int, dog ...DogHandler) *rsocketServer {
	return &rsocketServer{
		serverName: serverName,
		tcpAddress: tcpAddress,
		wssAddress: wssAddress,
		buffSize:   buffSize,
		dog:        dog,
	}
}

func (r *rsocketServer) Accept(route *Router) {
	r.serverBuilder = rsocket.Receive().OnStart(
		func() {
			r.wg.Done()
		},
	)

	//r.serverBuilder.Scheduler(
	//	scheduler.NewElastic(runtime.NumCPU()<<8),
	//	scheduler.NewElastic(runtime.NumCPU()<<8),
	//) // setting scheduler goroutine on numCPU*2 to better working

	r.serverBuilder.Resume()
	r.serverStart = r.serverBuilder.
		Acceptor(
			func(
				cc ctx.Context,
				setup payload.SetupPayload,
				sendingSocket rsocket.CloseableRSocket,
			) (rsocket.RSocket, error) {

				var c = getContext()
				var remoteIp, _ = rsocket.GetAddr(sendingSocket)

				if len(r.dog) > 0 {

					c.SetSetupData(setup.Data())

					for i := range r.dog {
						rsp, err := r.dog[i](c)
						if err != nil {
							c.Errorf("dog reject you |message=%s", c.Codec().MustEncodeString(rsp))
							return nil, err
						}
					}
				}

				return rsocket.NewAbstractSocket(
					setupFireAndForget(route, remoteIp, setup),
					setupRequestResponse(route, remoteIp, setup),
					setupRequestStream(route, remoteIp, setup),
					setupRequestChannel(route, remoteIp, r.buffSize, setup),
				), nil
			},
		)
}

func (r *rsocketServer) Run(wg *sync.WaitGroup) {
	r.wg = wg
	if r.tcpAddress != "" {
		wg.Add(1)
		r.tcp()
	}

	if r.wssAddress != "" {
		wg.Add(1)
		r.wss()
	}
}

// run tcp socket rsocketServer
func (r *rsocketServer) tcp() {
	go func() {
		err := r.serverStart.Transport(
			rsocket.
				TCPServer().
				SetAddr(r.tcpAddress).
				Build(),
		).Serve(ctx.TODO())

		if err != nil {
			panic(err)
		}
	}()
}

// run websocket rsocketServer
func (r *rsocketServer) wss() {
	go func() {
		err := r.serverStart.Transport(
			rsocket.
				WebsocketServer().
				SetAddr(r.wssAddress).
				Build(),
		).Serve(ctx.TODO())

		if err != nil {
			panic(err)
		}
	}()
}

// get metadata ignore error
func mustGetMetadata(p payload.Payload) []byte {
	b, _ := p.Metadata()
	return b
}

func setupRequestResponse(r *Router, remoteIp string, setup payload.SetupPayload) rsocket.OptAbstractSocket {
	return rsocket.RequestResponse(
		func(p payload.Payload) mono.Mono {

			c, err := fromMetadata(mustGetMetadata(p), setup.DataMimeType(), setup.MetadataMimeType())
			if err != nil {
				Fatalf("err=%v |metadata=%s |mimeType=%s", err, BytesToString(mustGetMetadata(p)), setup.MetadataMimeType())
				return mono.JustOneshot(payload.New(r.Error().Error400(c), nil))
			}

			var req, rsp = payload2avaPacket(p.Data()), newPacket()

			c.RemoteAddr = remoteIp

			err = r.RR(c, req, rsp)

			if err != nil && rsp.Len() > 0 {
				c.Error(err)
				return mono.JustOneshot(payload.New(rsp.Bytes(), nil))
			}

			if err != nil {
				c.Error(err)
				return mono.JustOneshot(payload.New(r.Error().Error400(c), nil))
			}
			Recycle(req)

			recycleContext(c)
			m := mono.JustOneshot(payload.New(rsp.Bytes(), nil))

			Recycle(rsp)
			return m
		},
	)
}

func setupFireAndForget(r *Router, remoteIp string, setup payload.SetupPayload) rsocket.OptAbstractSocket {
	return rsocket.FireAndForget(
		func(p payload.Payload) {

			var req = payload2avaPacket(p.Data())

			c, err := fromMetadata(mustGetMetadata(p), setup.DataMimeType(), setup.MetadataMimeType())
			if err != nil {
				Fatalf("err=%v |metadata=%s |mimeType=%s", err, BytesToString(mustGetMetadata(p)), setup.MetadataMimeType())
				return
			}

			c.RemoteAddr = remoteIp

			err = r.FF(c, req)

			if err != nil {
				c.Errorf("err=%v |path=%s", err, c.Metadata.Method())
				return
			}

			Recycle(req)

			recycleContext(c)
		},
	)
}

func (r *rsocketServer) Close() {
	return
}

func setupRequestStream(router *Router, remoteIp string, setup payload.SetupPayload) rsocket.OptAbstractSocket {
	return rsocket.RequestStream(
		func(p payload.Payload) flux.Flux {

			var exit = make(chan struct{})

			return flux.Create(
				func(ctx ctx.Context, sink flux.Sink) {

					var req = payload2avaPacket(p.Data())

					c, err := fromMetadata(mustGetMetadata(p), setup.DataMimeType(), setup.MetadataMimeType())
					if err != nil {
						Fatalf("err=%v |metadata=%s |mimeType=%s", err, BytesToString(mustGetMetadata(p)), setup.MetadataMimeType())
						return
					}

					c.RemoteAddr = remoteIp

					//if you want to Disconnect channel
					//you must close rsp from rsocketServer handler
					//this way is very friendly to closing channel transport
					rsp, err := router.RS(c, req, exit)

					//todo cannot know when socket will close to close(rsp)
					//you must close rsp at where send

					if err != nil {
						c.Errorf("transport RS failure |method=%s |err=%v", c.Metadata.Method(), err)
						return
					}

					for b := range rsp {
						data, e := c.Codec().Encode(b)
						if e != nil {
							c.Errorf("transport RS Encode failure |method=%s |err=%v", c.Metadata.Method(), err)
							continue
						}

						sink.Next(payload.New(data, nil))
					}
					sink.Complete()

					Recycle(req)
				},
			).DoOnError(func(e error) {
				Debugf("setup setupRequestStream OnError |err=%v", e)
			}).DoOnComplete(func() {
				Debug("setup setupRequestStream OnComplete")
			}).DoFinally(func(s rx.SignalType) {
				exit <- struct{}{}
				close(exit)
				Debug("setup setupRequestStream DoFinally")
			}).DoOnSubscribe(func(ctx ctx.Context, s rx.Subscription) {
				Debug("setup setupRequestStream DoOnSubscribe")
			})
		},
	)
}

func setupRequestChannel(router *Router, remoteIp string, buffSize int, setup payload.SetupPayload) rsocket.OptAbstractSocket {
	return rsocket.RequestChannel(
		func(f flux.Flux) flux.Flux {
			var (
				req  = make(chan *Packet, buffSize)
				exit = make(chan struct{})
			)

			//read data from client by channel transport method
			f.SubscribeOn(scheduler.Parallel()).
				DoFinally(
					func(s rx.SignalType) {
						close(req)
					},
				).
				Subscribe(
					ctx.Background(),
					rx.OnNext(
						func(p payload.Payload) error {
							req <- payload2avaPacket(payload.Clone(p).Data())
							return nil
						},
					),
					rx.OnError(
						//if client is occurred error
						func(e error) {
							Errorf("setupRequestChannel OnError |err=%v", e)
						},
					),
				)

			return flux.Create(
				func(ctx ctx.Context, sink flux.Sink) {

					var meta []byte
					for b := range req {
						meta = b.Bytes()
						//Debugf("requestChanel success |ip=%s |meta=%s", remoteIp, string(meta))
						break
					}

					c, err := fromMetadata(meta, setup.DataMimeType(), setup.MetadataMimeType())
					if err != nil {
						Errorf("err=%v |metadata=%s |mimeType=%s", err, BytesToString(meta), setup.MetadataMimeType())
						return
					}

					c.RemoteAddr = remoteIp

					//if you want to Disconnect channel
					//you must close rsp from rsocketServer handler
					//this way is very friendly to closing channel transport
					rsp, err := router.RC(c, req, exit)
					if err != nil {
						c.Errorf("transport RC failure |method=%s |err=%v", c.Metadata.Method(), err)
						return
					}

					for b := range rsp {
						data, e := c.Codec().Encode(b)
						if e != nil {
							c.Errorf("transport RC Encode failure |method=%s |err=%v", c.Metadata.Method(), err)
							continue
						}
						sink.Next(payload.New(data, nil))
					}
					sink.Complete()

				},
			).DoOnError(func(e error) {
				Debugf("setup setupRequestChannel OnError |err=%v", e)
			}).DoOnComplete(func() {
				Debug("setup setupRequestChannel OnComplete")
			}).DoFinally(func(s rx.SignalType) {
				exit <- struct{}{}
				close(exit)
				Debug("setup setupRequestChannel DoFinally")
			})
		},
	)
}
