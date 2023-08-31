package main

import (
	"log"
	"net/rpc"
	"flag"
)

type Response struct {
	Status string
}
type Request struct {}

func main() {
	sway := flag.Bool("sway", false, "sway operation")
	flag.Parse()
	var rpcEndpoint string
	if *sway {
		rpcEndpoint = "/tmp/swayjumplist-socket"
	} else {
		rpcEndpoint = "/tmp/i3jumplist-socket"
	}

	client, err := rpc.Dial("unix", rpcEndpoint)
	if err != nil {
		log.Fatalf("failed: %s", err)
	}

	req := &Request{}
	var res Response

	err = client.Call("JumplistNav.Back", req, &res)
	if err != nil {
		log.Fatalf("error in rpc: %s", err)
	}

	log.Println(res.Status)
}
