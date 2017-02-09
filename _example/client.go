package main

import (
	"fmt"
	"log"

	"github.com/nobonobo/jsonrpc"
)

func main() {
	client, err := jsonrpc.NewClient("http://localhost:8080/jsonrpc", nil)
	if err != nil {
		log.Fatalln(err)
	}
	args := []int{5, 8}
	reply := 0
	if err := client.Call("Sample.Add", &args, &reply); err != nil {
		log.Fatalln(err)
	}
	fmt.Println("reply:", reply)
}
