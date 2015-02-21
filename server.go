package jsonrpc

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	jsonrpcorg "net/rpc/jsonrpc"
	"sync"
)

const Connected = "200 Connected to JSON RPC"

// DefaultRPCPath ...
var DefaultRPCPath = "/jsonrpc"

type codecWrapper struct {
	rpc.ServerCodec
}

func (c *codecWrapper) WriteResponse(r *rpc.Response, x interface{}) error {
	defer c.ServerCodec.Close()
	return c.ServerCodec.WriteResponse(r, x)
}

// Server ...
type Server struct {
	*rpc.Server
	pool *sync.Pool
}

// NewServer ...
func NewServer() *Server {
	server := &Server{Server: rpc.NewServer(), pool: &sync.Pool{}}
	return server
}

// ServeHTTP implements an http.Handler that answers RPC requests.
func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "CONNECT":
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			log.Print("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
			return
		}
		io.WriteString(conn, "HTTP/1.0 "+Connected+"\n\n")
		server.ServeCodec(jsonrpcorg.NewServerCodec(conn))
	case "POST":
		dst, src := net.Pipe()
		go io.Copy(src, req.Body)
		go server.ServeCodec(&codecWrapper{jsonrpcorg.NewServerCodec(dst)})
		io.Copy(w, src)
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "405 POST or CONNECT\n")
	}
}
