package main

import (
	"log"
	"net/http"

	"github.com/nobonobo/jsonrpc"
)

type Sample struct{}

func (s *Sample) Add(args *[]int, reply *int) error {
	*reply = 0
	for _, v := range *args {
		*reply += v
	}
	return nil
}

func main() {
	server := jsonrpc.NewServer()
	server.Register(&Sample{})
	http.Handle(jsonrpc.DefaultRPCPath, server)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalln(err)
	}
}
