package stream

import (
	"fmt"
	"github.com/aiakit/ava"
	"github.com/aiakit/ava/_example/tutorial/internal/ipc"
	"github.com/aiakit/ava/_example/tutorial/proto/phello"
	"sync/atomic"
)

// call to srv.hello

type Hello struct{}

func (h *Hello) Hi(c *ava.Context, req *phello.SayReq, rsp *phello.SayRsp) {
	rspCh := ipc.SayStream(c, &phello.SayReq{Ping: "ping"})

	var count uint32

	var done = make(chan struct{})
	go func() {
	QUIT:
		for {
			select {
			case b, ok := <-rspCh:
				if ok {
					fmt.Println("------receive from srv.hello----", b.Pong)
					atomic.AddUint32(&count, 1)
				} else {
					break QUIT
				}
			}
		}
		done <- struct{}{}

		fmt.Println("say handler count is: ", atomic.LoadUint32(&count))
	}()
	<-done

	rsp.Pong = "pong"
}
