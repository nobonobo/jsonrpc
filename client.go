package jsonrpc

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/rpc"
	jsonrpcorg "net/rpc/jsonrpc"
	"net/url"
)

// NewClientLocal ...
func NewClientLocal() *rpc.Client {
	cl, cr := net.Pipe()
	go ServeCodec(jsonrpcorg.NewServerCodec(cl))
	return jsonrpcorg.NewClient(cr)
}

// NewClientHTTP ...
func NewClientHTTP(endpoint string) *rpc.Client {
	buf := bytes.NewBuffer(nil)
	r, w := io.Pipe()
	return rpc.NewClientWithCodec(&rpcCodec{
		ClientCodec: jsonrpcorg.NewClientCodec(struct {
			io.Writer
			io.ReadCloser
		}{buf, r}),
		endpoint: endpoint,
		buf:      buf,
		writer:   w,
		ch:       make(chan *http.Response, 1),
	})
}

type rpcCodec struct {
	rpc.ClientCodec
	endpoint string
	buf      *bytes.Buffer
	writer   io.Writer
	ch       chan *http.Response
}

func (c *rpcCodec) WriteRequest(r *rpc.Request, v interface{}) error {
	c.buf.Reset()
	if err := c.ClientCodec.WriteRequest(r, v); err != nil {
		return err
	}
	resp, err := http.Post(c.endpoint, "application/json-rpc", c.buf)
	if err != nil {
		return err
	}
	c.ch <- resp
	return nil
}

func (c *rpcCodec) ReadResponseHeader(r *rpc.Response) error {
	resp := <-c.ch
	if resp != nil {
		go func() {
			io.Copy(c.writer, resp.Body)
			resp.Body.Close()
		}()
	}
	return c.ClientCodec.ReadResponseHeader(r)
}

// DialHTTP connected Client
func DialHTTP(endpoint string, config *tls.Config) (*rpc.Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	var conn net.Conn
	path := DefaultRPCPath
	switch u.Scheme {
	case "unix":
		conn, err = net.Dial("unix", u.Opaque+u.Path)
		if err != nil {
			return nil, err
		}
	case "http":
		conn, err = net.Dial("tcp", u.Host)
		if err != nil {
			return nil, err
		}
		path = u.Path
	case "https":
		conn, err = tls.Dial("tcp", u.Host, config)
		if err != nil {
			return nil, err
		}
		path = u.Path
	default:
		return nil, fmt.Errorf("not supported scheme: %q", u.Scheme)
	}
	io.WriteString(conn, "CONNECT "+path+" HTTP/1.0\n\n")

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == Connected {
		return jsonrpcorg.NewClient(conn), nil
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	conn.Close()
	return nil, &net.OpError{
		Op:   "dial-http",
		Net:  "tcp " + u.Host,
		Addr: nil,
		Err:  err,
	}
}
