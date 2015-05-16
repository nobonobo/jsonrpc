package jsonrpc

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	org "net/rpc/jsonrpc"
	"net/url"
	"runtime"
)

var DefaultPoolConnections = runtime.NumCPU()

// RemoteError JSON-RPCにかかわらないエラータイプ
type RemoteError error

// DialHTTP connects
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
		return org.NewClient(conn), nil
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

type Client struct {
	pool    chan *conn
	newFunc func() *conn
}

type conn struct {
	*rpc.Client
	Error error
}

func NewClient(endpoint string, config *tls.Config) *Client {
	client := new(Client)
	client.pool = make(chan *conn, DefaultPoolConnections)
	client.newFunc = func() *conn {
		c, err := DialHTTP(endpoint, config)
		return &conn{Client: c, Error: err}
	}
	return client
}

func (client *Client) get() *conn {
	select {
	case c, ok := <-client.pool:
		if ok {
			return c
		}
	default:
	}
	return client.newFunc()
}

func (client *Client) put(c *conn) {
	select {
	case client.pool <- c:
	default:
		c.Close()
	}
}

func (client *Client) fail(serviceMethod string, args interface{}, reply interface{}, done chan *rpc.Call, err error) *rpc.Call {
	call := new(rpc.Call)
	call.ServiceMethod = serviceMethod
	call.Args = args
	call.Reply = reply
	if done == nil {
		done = make(chan *rpc.Call, 1)
	}
	call.Done = done
	call.Error = err
	select {
	case call.Done <- call:
	default:
	}
	return call
}

func (client *Client) Go(serviceMethod string, args interface{}, reply interface{}, done chan *rpc.Call) *rpc.Call {
	if done != nil && cap(done) == 0 {
		log.Panic("rpc: done channel is unbuffered")
	}
	c := client.get()
	if c == nil {
		return client.fail(serviceMethod, args, reply, done, fmt.Errorf("closed"))
	}
	if c.Error != nil {
		return client.fail(serviceMethod, args, reply, done, c.Error)
	}
	call := c.Go(serviceMethod, args, reply, nil)
	if call.Error != nil {
		return call
	}
	cc := *call
	cc.Done = done
	go func() {
		r, ok := <-call.Done
		if ok {
			if r.Error == nil {
				client.put(c)
			} else {
				_, ok := r.Error.(RemoteError)
				if ok {
					client.put(c)
				} else {
					c.Close()
				}
			}
			r.Done = done
			select {
			case done <- r:
			default:
			}
		}
	}()
	return &cc
}

func (client *Client) Call(serviceMethod string, args interface{}, reply interface{}) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *rpc.Call, 1)).Done
	return call.Error
}

func (client *Client) Close() error {
	var last error
	close(client.pool)
	for c := range client.pool {
		if err := c.Close(); err != nil {
			last = err
		}
	}
	return last
}

func (client *Client) Get(prefix string) Service {
	return &service{Client: client, prefix: prefix + "."}
}

type Service interface {
	Call(serviceMethod string, args interface{}, reply interface{}) error
	Go(serviceMethod string, args interface{}, reply interface{}, done chan *rpc.Call) *rpc.Call
	Close() error
}

type service struct {
	*Client
	prefix string
}

func (s *service) Call(serviceMethod string, args interface{}, reply interface{}) error {
	return s.Client.Call(s.prefix+serviceMethod, args, reply)
}

func (s *service) Go(serviceMethod string, args interface{}, reply interface{}, done chan *rpc.Call) *rpc.Call {
	return s.Client.Go(s.prefix+serviceMethod, args, reply, done)
}
