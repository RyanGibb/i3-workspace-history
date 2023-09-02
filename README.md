# i3-workspace-history

Is `workspace back_and_forth` not enough?
Do you want to navigate to that workspace you were at two, three, or more, workspaces ago?
Inspired by Vim's jumplist, this program allows traversing your i3 or sway workspace history.

The server subscribes to the i3/sway IPC and listens for workspace events, and maintains a list of workspace visited.
The back/forward modes use an RPC over a Unix domain socket to communicate to the server, which then uses the i3/sway IPC interface to switch workspace.

The history works like vim's jumplist, so while traversing it the list stays constant.
If you switch workspace while traversing the history, the history will not be truncated, rather the new workspace will be appended to the history.

To enable sway support, invoke the executables with `-sway` as a command line argument.
Different domain sockets are used for i3 and sway, so this should work for both running at the same time, though this is untested.

You might find the following configuration useful:
```
bindsym $mod+i exec i3-workspace-history -mode=forward
bindsym $mod+o exec i3-workspace-history -mode=back
exec i3-workspace-history
```

This project can be built using Nix with `nix build .`, or go with `go build i3-workspace-history.go`.
