package main

import (
	"fmt"
	"log"

	"github.com/nobonobo/jsonrpc"
)

func main() {
	svc := jsonrpc.NewClient("http://localhost:8080/jsonrpc", nil).Get("Sample")
	args := []int{5, 8}
	reply := 0
	if err := svc.Call("Add", &args, &reply); err != nil {
		log.Fatalln(err)
	}
	fmt.Println("reply:", reply)
}
