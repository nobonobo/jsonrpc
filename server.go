package jsonrpc

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/rpc"
	jsonrpcorg "net/rpc/jsonrpc"
)

// Connected Response String for CONNECT/HTTP
const Connected = "200 Connected to JSON RPC"

var (
	// DefaultRPCPath ...
	DefaultRPCPath = "/jsonrpc"
	// DefaultServer ...
	DefaultServer = NewServer()
)

// Server ...
type Server struct {
	*rpc.Server
}

// NewServer ...
func NewServer() *Server {
	return &Server{rpc.NewServer()}
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
		dst := &struct {
			io.ReadCloser
			io.Writer
		}{
			ioutil.NopCloser(req.Body),
			w,
		}
		server.ServeRequest(jsonrpcorg.NewServerCodec(dst))
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "405 POST or CONNECT\n")
	}
}

// HandleHTTP ...
func HandleHTTP() {
	http.Handle(DefaultRPCPath, DefaultServer)
}

// Register ...
func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

// ServeCodec ...
func ServeCodec(codec rpc.ServerCodec) {
	DefaultServer.ServeCodec(codec)
}
