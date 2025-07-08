package hijack

import (
	"github.com/aiakit/ava"
	"net/http"
)

func HijackWriter(c *ava.Context, r *http.Request, w http.ResponseWriter, req, rsp *ava.Packet) bool {
	w.Write(rsp.Bytes())
	return true
}
