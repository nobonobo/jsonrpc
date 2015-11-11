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
	"sync"
)

var (
	None = &struct{}{}
)

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

// Client ...
type Client struct {
	sync.RWMutex
	newFunc func() (*rpc.Client, error)
	c       *rpc.Client
	closing bool
}

// NewClient ...
func NewClient(endpoint string, config *tls.Config) *Client {
	client := new(Client)
	client.newFunc = func() (*rpc.Client, error) {
		return DialHTTP(endpoint, config)
	}
	return client
}

func (client *Client) conn() (*rpc.Client, error) {
	client.RLock()
	c, closing := client.c, client.closing
	client.RUnlock()
	if closing {
		return nil, fmt.Errorf("closed")
	}
	if c != nil {
		return c, nil
	}
	client.Lock()
	defer client.Unlock()
	c, err := client.newFunc()
	if err != nil {
		return nil, err
	}
	client.c = c
	return c, nil
}

// Go ...
func (client *Client) Go(serviceMethod string, args interface{}, reply interface{}, done chan *rpc.Call) *rpc.Call {
	if done == nil {
		done = make(chan *rpc.Call, 1)
	} else if cap(done) == 0 {
		log.Panic("rpc: done channel is unbuffered")
	}
	c, err := client.conn()
	if err != nil {
		call := &rpc.Call{
			ServiceMethod: serviceMethod,
			Args:          args,
			Reply:         reply,
			Done:          done,
			Error:         err,
		}
		call.Done <- call
		return call
	}
	pre := make(chan *rpc.Call, 1)
	call := c.Go(serviceMethod, args, reply, pre)
	if call.Error != nil {
		return call
	}
	call.Done = done
	go func() {
		r, ok := <-pre
		if !ok {
			return
		}
		if r.Error != nil {
			if _, ok := r.Error.(rpc.ServerError); ok {
				client.Lock()
				client.c.Close()
				client.c = nil
				client.Unlock()
			}
		}
		select {
		case done <- r:
		default:
		}
		client.Lock()
		if client.closing {
			client.c.Close()
			client.c = nil
		}
		client.Unlock()
	}()
	return call
}

// Call ...
func (client *Client) Call(serviceMethod string, args interface{}, reply interface{}) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *rpc.Call, 1)).Done
	return call.Error
}

// Close ...
func (client *Client) Close() error {
	client.Lock()
	defer client.Unlock()
	client.closing = true
	return nil
}

// Get ...
func (client *Client) Get(prefix string) Service {
	return &service{Client: client, prefix: prefix + "."}
}

// Service ...
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
