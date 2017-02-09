package jsonrpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
	server := NewServer()
	server.Register(&Sample{})
	mux.Handle(DefaultRPCPath, server)
	endpoint := hs.URL + DefaultRPCPath

	client := NewClient(endpoint, nil)
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
	server := NewServer()
	server.Register(&Sample{})
	mux.Handle(DefaultRPCPath, server)
	endpoint := hs.URL + DefaultRPCPath

	client := NewClient(endpoint, nil)
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
	server := NewServer()
	server.Register(&Sample{})
	mux.Handle(DefaultRPCPath, server)
	endpoint := hs.URL + DefaultRPCPath

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			args := []int{5 * n, 8 * n}
			js, _ := json.Marshal([1]interface{}{args})
			resp, err := http.Post(endpoint, "", strings.NewReader(fmt.Sprintf(`{"id":%d,"method":"Sample.Add","params":%s}`, n+1, js)))
			if err != nil {
				t.Log(err)
				t.FailNow()
			}
			defer resp.Body.Close()
			var reply struct {
				Result int `json:"result"`
			}
			json.NewDecoder(resp.Body).Decode(&reply)
			t.Log(n, ": reply =", reply.Result)
		}(i)
	}
	wg.Wait()
}
