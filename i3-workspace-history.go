package main

// adapted from https://gist.github.com/adamws/6109404639b554a3e18fac33cd1ca68f

import (
	"fmt"
	"net"
	"net/rpc"
	"go.i3wm.org/i3/v4"
	"log"
	"os"
	"os/exec"
	"bytes"
	"flag"
)

var jumplist []string
var pointer int
var next string
var going bool

type Response struct {
	Status string
}
type Request struct {}
type JumplistNav struct{}

func (*JumplistNav) Back(req Request, res *Response) (err error) {
	if len(jumplist) < 1 {
		res.Status = "ignoring client due to empty jumplist"
		return
	}
	if pointer >= len(jumplist) - 1 {
		res.Status = "at end of history"
		return
	}
	pointer++
	index := len(jumplist) - 1 - pointer
	next = jumplist[index]
	going = true
	log.Printf("go to: %s", next)

	_, err = i3.RunCommand(fmt.Sprintf("workspace %s", next))
	if err != nil && !i3.IsUnsuccessful(err) {
		res.Status = fmt.Sprintf("i3.RunCommand() failed with %s\n", err)
		return err
	}

	res.Status = "ok"
	return
}

func (*JumplistNav) Forward(req Request, res *Response) (err error) {
	if len(jumplist) < 1 {
		res.Status = "ignoring client due to empty jumplist"
		return
	}
	if pointer <= 0 {
		res.Status = "at end of history"
		return
	}
	pointer--
	index := len(jumplist) - 1 - pointer
	next = jumplist[index]
	going = true
	log.Printf("go to: %s", next)

	_, err = i3.RunCommand(fmt.Sprintf("workspace %s", next))
	if err != nil && !i3.IsUnsuccessful(err) {
		res.Status = fmt.Sprintf("i3.RunCommand() failed with %s\n", err)
		return err
	}

	res.Status = "ok"
	return
}

func server(sway bool, rpcEndpoint string) {
	if sway {
		i3.SocketPathHook = func() (string, error) {
			out, err := exec.Command("sway", "--get-socketpath").CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("getting sway socketpath: %v (output: %s)", err, out)
			}
			return string(out), nil
		}
	
		i3.IsRunningHook = func() bool {
			out, err := exec.Command("pgrep", "-c", "sway\\$").CombinedOutput()
			if err != nil {
				log.Printf("sway running: %v (output: %s)", err, out)
			}
			return bytes.Compare(out, []byte("1")) == 0
		}
	}

	go func() {
		recv := i3.Subscribe(i3.WorkspaceEventType)
		for recv.Next() {
			ev := recv.Event().(*i3.WorkspaceEvent)
			if ev.Change == "focus" {
				if going && next == ev.Current.Name {
					going = false
					log.Printf("gone to: %s", next)
				} else {
					for i, e := range jumplist {
						if e == ev.Current.Name {
							jumplist = append(jumplist[:i], jumplist[i+1:]...)
							break
						}
					}
					jumplist = append(jumplist, ev.Current.Name)
					pointer = 0
				}
				log.Printf("current jumplist: %s", jumplist)
				log.Printf("current pointer: %s", pointer)
			}
		}
	}()

	pointer = 0
	rpc.Register(&JumplistNav{})
	os.Remove(rpcEndpoint)
	listener, err := net.Listen("unix", rpcEndpoint)
	if err != nil {
		log.Fatalf("unable to listen at %s: %s", rpcEndpoint, err)
	}

	go rpc.Accept(listener)

	select{}
}

func forward(sway bool, rpcEndpoint string) {
	client, err := rpc.Dial("unix", rpcEndpoint)
	if err != nil {
		log.Fatalf("failed: %s", err)
	}

	req := &Request{}
	var res Response

	err = client.Call("JumplistNav.Forward", req, &res)
	if err != nil {
		log.Fatalf("error in rpc: %s", err)
	}

	log.Println(res.Status)
}

func back(sway bool, rpcEndpoint string) {
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

func main() {
	sway := flag.Bool("sway", false, "Sway operation.")
	mode := flag.String("mode", "server", "Either server, back, or forward.")
	flag.Parse()
	var rpcEndpoint string
	if *sway {
		rpcEndpoint = "/tmp/swayjumplist-socket"
	} else {
		rpcEndpoint = "/tmp/i3jumplist-socket"
	}

	switch *mode {
		case "server":
			server(*sway, rpcEndpoint)
		case "back":
			back(*sway, rpcEndpoint)
		case "forward":
			forward(*sway, rpcEndpoint)
		default:
			fmt.Fprintln(os.Stderr, "Mode must be one of server, back, or forward.\n\n" +
				"Usage: i3-workspace-history")
			os.Exit(1)
	}
}
