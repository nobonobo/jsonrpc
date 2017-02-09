package jsonrpc

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type Sample struct{}

func (s *Sample) Add(args *[]int, reply *int) error {
	*reply = 0
	for _, v := range *args {
		*reply += v
	}
	return nil
}

func TestJsonrpcSync(t *testing.T) {
	mux := http.NewServeMux()
	hs := httptest.NewServer(mux)
	defer hs.Listener.Close()
	Register(&Sample{})
	mux.Handle(DefaultRPCPath, DefaultServer)
	endpoint := hs.URL + DefaultRPCPath

	client, err := DialHTTP(endpoint, nil)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	defer client.Close()
	for i := 0; i < 10; i++ {
		args := []int{5 * i, 8 * i}
		reply := 0
		if err := client.Call("Sample.Add", &args, &reply); err != nil {
			t.Log(err)
		}
		t.Log(i, ": reply =", reply)
	}
}

func TestJsonrpcAsync(t *testing.T) {
	mux := http.NewServeMux()
	hs := httptest.NewServer(mux)
	defer hs.Listener.Close()
	Register(&Sample{})
	mux.Handle(DefaultRPCPath, DefaultServer)
	endpoint := hs.URL + DefaultRPCPath

	client, err := DialHTTP(endpoint, nil)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	defer client.Close()
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			args := []int{5 * n, 8 * n}
			reply := 0
			if err := client.Call("Sample.Add", &args, &reply); err != nil {
				t.Log(err)
			}
			t.Log(n, ": reply =", reply)
		}(i)
	}
	wg.Wait()
}

func TestJsonrpcPost(t *testing.T) {
	mux := http.NewServeMux()
	hs := httptest.NewServer(mux)
	defer hs.Listener.Close()
	Register(&Sample{})
	mux.Handle(DefaultRPCPath, DefaultServer)
	endpoint := hs.URL + DefaultRPCPath

	client := NewClientHTTP(endpoint)
	defer client.Close()
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			args := []int{5 * n, 8 * n}
			reply := 0
			if err := client.Call("Sample.Add", &args, &reply); err != nil {
				t.Log(err)
			}
			t.Log(n, ": reply =", reply)
		}(i)
	}
	wg.Wait()
}

func TestJsonrpcLocal(t *testing.T) {
	Register(&Sample{})

	client := NewClientLocal()
	defer client.Close()
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			args := []int{5 * n, 8 * n}
			reply := 0
			if err := client.Call("Sample.Add", &args, &reply); err != nil {
				t.Log(err)
			}
			t.Log(n, ": reply =", reply)
		}(i)
	}
	wg.Wait()
}
