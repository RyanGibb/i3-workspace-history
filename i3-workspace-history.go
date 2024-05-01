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
	"strings"
)

var jumplist []string
var index int
var next string
var navigating bool
var start_navigating bool
var notify_id string

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
	if index <= 0 {
		res.Status = "at end of history"
		return
	}
	index--;
	next = jumplist[index]
	log.Printf("go to: %s", next)

	navigating = true
	if index == len(jumplist) - 1 {
		start_navigating = true
	}

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
	if index >= len(jumplist) - 1 {
		res.Status = "at end of history"
		return
	}
	index++
	next = jumplist[index]
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
				log.Printf( notify_id)
				out, err := exec.Command("notify-send", "-t", "500", "-p", "-r", notify_id, "workspace: " + ev.Current.Name).CombinedOutput()
				if err != nil {
					log.Printf("notify-send: %v (output: %s)", err, out)
				} else {
					notify_id = strings.TrimSpace(string(out))
				}

				if navigating && next != ev.Current.Name {
					navigating = false
					start_navigating = false
					index = len(jumplist)
					log.Printf("no longer navigating history")
				} else if !navigating || start_navigating {
					for i, e := range jumplist {
						if e == ev.Old.Name {
							jumplist = append(jumplist[:i], jumplist[i+1:]...)
							if start_navigating {
								index--
							}
							break
						}
					}
					jumplist = append(jumplist, ev.Old.Name)
					if !start_navigating {
						index = len(jumplist)
					} else {
						start_navigating = false
					}
				}
				log.Printf("current jumplist: %s", jumplist)
				log.Printf("current index: %s", index)
			}
		}
	}()

	index = 0
	// some unlikely id
	notify_id = "9999999"
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
