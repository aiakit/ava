package hello

import (
	"fmt"
	"github.com/aiakit/ava"
	"github.com/aiakit/ava/_example/tutorial/proto/phello"
	"strconv"
	"sync/atomic"
	"time"
)

type Say struct{}

// SayGet if err!=nil header status code will be 500
// bench shell
/*
wrk -t100 -c1000 -d10s --latency http://127.0.0.1:10000/hello/saysrv/say
  100 threads and 1000 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    13.45ms   12.17ms 248.27ms   80.96%
    Req/Sec   456.48    316.67     3.18k    81.65%
  Latency Distribution
     50%   11.25ms
     75%   17.81ms
     90%   24.66ms
     99%   62.24ms
  412262 requests in 10.10s, 47.18MB read
  Socket errors: connect 0, read 886, write 0, timeout 0
Requests/sec:  40821.21
Transfer/sec:      4.67MB
*/

func (h *Say) Say(c *ava.Context, req *phello.SayReq, rsp *phello.SayRsp) {
	c.Infof("req=%v", req)
	rsp.Pong = "pong"
}

func (h *Say) Stream(c *ava.Context, req *phello.SayReq, exit chan struct{}) chan *phello.SayRsp {
	var rsp = make(chan *phello.SayRsp)

	go func() {
		var count uint32
		for i := 0; i < 5; i++ {
			rsp <- &phello.SayRsp{Pong: strconv.Itoa(i)}
			atomic.AddUint32(&count, 1)
			time.Sleep(time.Millisecond * 500)
		}

		c.Info("say stream example count is: ", atomic.LoadUint32(&count))

		close(rsp)
	}()

	return rsp
}

func (h *Say) Channel(c *ava.Context, req chan *phello.SayReq, exit chan struct{}) chan *phello.SayRsp {
	var rsp = make(chan *phello.SayRsp)

	go func() {
	QUIT:
		for {
			select {
			case data, ok := <-req:
				if !ok {
					break QUIT
				}

				fmt.Println("----", data.Ping)
				//test channel sending frequency
				time.Sleep(time.Second)
				rsp <- &phello.SayRsp{Pong: data.Ping}
			case <-exit:
				break QUIT
			}
		}

		close(rsp)
	}()

	return rsp
}
