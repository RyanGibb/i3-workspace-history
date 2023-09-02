# i3-workspace-history

Inspired by Vim's jumplist, this program allows traversing your i3 or sway workspace history.

The server subscribes to the IPC and listens for workspace events, and maintains a history.
The back/forward executables use an RPC over a Unix domain socket to communicate to the server to use the IPC interface to switch workspace.

The jumplist history works like vim's, so while traversing the list stays constant.
If you switch workspace using normal mechanisms while traversing the history, the history will not be truncated, rather we will back to the end of the history and the new workspace will be appended.

To enable sway support, invoke the executables with `-sway` as a command line argument.
Different domain sockets are used for i3 and sway, so this should work for both running at the same time, though this is untested.

You might find the following configuration useful:
```
bindsym $mod+i exec i3-workspace-history/bin/forward
bindsym $mod+o exec i3-workspace-history/bin/back
exec i3-workspace-history/bin/server
```

With the appropriate path to the project.

This project can be built using Nix flakes `nix build .`, or using go.
