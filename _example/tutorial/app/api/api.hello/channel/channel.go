package apichannel

import (
	"fmt"
	"github.com/aiakit/ava"
	"github.com/aiakit/ava/_example/tutorial/internal/ipc"
	"github.com/aiakit/ava/_example/tutorial/proto/phello"
	"strconv"
	"sync/atomic"
	"time"
)

// call to srv.hello

type Hello struct{}

func (h *Hello) Hi(c *ava.Context, req *phello.SayReq, rsp *phello.SayRsp) {
	var reqCh = make(chan *phello.SayReq, 100)
	go func() {
		for i := 0; i < 3; i++ {

			//test sending frequency
			time.Sleep(time.Second)
			reqCh <- &phello.SayReq{Ping: strconv.Itoa(i)}

			//if i == 20 {
			//	errsIn <- errors.SetupService("send a test error")
			//	break
			//}
		}

		close(reqCh)
	}()

	rspCh := ipc.SayChannel(c, reqCh)

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
