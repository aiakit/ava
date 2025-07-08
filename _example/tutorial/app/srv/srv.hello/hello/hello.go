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
					//just break select
					break
				}

				fmt.Println("----", data.Ping)
				//test channel sending frequency
				time.Sleep(time.Second)
				rsp <- &phello.SayRsp{Pong: data.Ping}
			case <-exit:
				//do something when channel quit
				break QUIT
			}
		}

		close(rsp)
	}()

	return rsp
}
