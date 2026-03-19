type workspace = Num of int | Name of string

let workspace_equal a b =
  match (a, b) with
  | Num x, Num y -> x = y
  | Name x, Name y -> String.equal x y
  | _ -> false

let pp_workspace = function Num n -> string_of_int n | Name s -> s

type state = {
  jumplist : workspace list;
  index : int;
  next_ws : workspace option;
  navigating : bool;
  start_navigating : bool;
}

let extract_workspace ws_name =
  let len = String.length ws_name in
  let rec count_digits i =
    if i >= len then i
    else match ws_name.[i] with '0' .. '9' -> count_digits (i + 1) | _ -> i
  in
  match count_digits 0 with
  | 0 -> Name ws_name
  | n -> Num (int_of_string (String.sub ws_name 0 n))

let workspace_command = function
  | Num n -> Printf.sprintf "workspace number %d" n
  | Name s -> Printf.sprintf "workspace %s" s

let remove_first pred lst =
  let rec loop acc = function
    | [] -> (List.rev acc, false)
    | x :: rest when pred x -> (List.rev_append acc rest, true)
    | x :: rest -> loop (x :: acc) rest
  in
  loop [] lst

let handle_focus state current_name old_name =
  let current_ws = extract_workspace current_name in
  let old_ws = extract_workspace old_name in
  if
    state.navigating
    && not
         (Option.fold ~none:false
            ~some:(workspace_equal current_ws)
            state.next_ws)
  then
    {
      state with
      navigating = false;
      start_navigating = false;
      index = List.length state.jumplist;
    }
  else if (not state.navigating) || state.start_navigating then
    let jumplist, removed =
      remove_first (workspace_equal old_ws) state.jumplist
    in
    let index =
      if removed && state.start_navigating then state.index - 1 else state.index
    in
    let jumplist = jumplist @ [ old_ws ] in
    {
      state with
      jumplist;
      index = (if state.start_navigating then index else List.length jumplist);
      start_navigating = false;
    }
  else state

let handle_back state =
  let len = List.length state.jumplist in
  if len < 1 then (state, "ignoring client due to empty jumplist", None)
  else if state.index <= 0 then (state, "at end of history", None)
  else
    let i = state.index - 1 in
    let ws = List.nth state.jumplist i in
    ( {
        state with
        index = i;
        next_ws = Some ws;
        navigating = true;
        start_navigating = i = len - 1;
      },
      "ok",
      Some ws )

let handle_forward state =
  let len = List.length state.jumplist in
  if len < 1 then (state, "ignoring client due to empty jumplist", None)
  else if state.index >= len - 1 then (state, "at end of history", None)
  else
    let i = state.index + 1 in
    let ws = List.nth state.jumplist i in
    ({ state with index = i; next_ws = Some ws }, "ok", Some ws)

let log fmt = Printf.ksprintf (fun s -> Printf.eprintf "%s\n%!" s) fmt

let read_exact fd n =
  let buf = Bytes.create n in
  let rec loop off =
    if off >= n then ()
    else
      let r = Unix.read fd buf off (n - off) in
      if r = 0 then failwith "connection closed";
      loop (off + r)
  in
  loop 0;
  Bytes.to_string buf

let send_i3_msg fd typ payload =
  let buf = Buffer.create (14 + String.length payload) in
  Buffer.add_string buf "i3-ipc";
  List.iter
    (fun n ->
      List.iter
        (fun s -> Buffer.add_char buf (Char.chr ((n lsr s) land 0xff)))
        [ 0; 8; 16; 24 ])
    [ String.length payload; typ ];
  Buffer.add_string buf payload;
  let s = Buffer.contents buf in
  ignore (Unix.write_substring fd s 0 (String.length s))

let recv_i3_msg fd =
  let h = read_exact fd 14 in
  assert (String.sub h 0 6 = "i3-ipc");
  let u32 off =
    let b i = Char.code h.[off + i] in
    b 0 lor (b 1 lsl 8) lor (b 2 lsl 16) lor (b 3 lsl 24)
  in
  (u32 10, read_exact fd (u32 6))

let connect_i3 path =
  let fd = Unix.socket Unix.PF_UNIX Unix.SOCK_STREAM 0 in
  Unix.connect fd (Unix.ADDR_UNIX path);
  fd

let get_socket_path sway =
  let env = if sway then "SWAYSOCK" else "I3SOCK" in
  let cmd = if sway then "sway --get-socketpath" else "i3 --get-socketpath" in
  (try Sys.getenv env
   with Not_found ->
     let ic = Unix.open_process_in cmd in
     Fun.protect
       ~finally:(fun () -> ignore (Unix.close_process_in ic))
       (fun () -> input_line ic))
  |> String.trim

let parse_workspace_event json_str =
  let open Yojson.Safe.Util in
  let json = Yojson.Safe.from_string json_str in
  match json |> member "change" |> to_string with
  | "focus" -> (
      let current = json |> member "current" |> member "name" |> to_string in
      match json |> member "old" with
      | `Null -> None
      | old -> Some (current, old |> member "name" |> to_string))
  | _ -> None

let process_i3_event state evt_fd =
  let msg_type, payload = recv_i3_msg evt_fd in
  if msg_type land 0x80000000 <> 0 then
    match parse_workspace_event payload with
    | Some (current, old) ->
        let state = handle_focus state current old in
        log "current jumplist: [%s]"
          (state.jumplist |> List.map pp_workspace |> String.concat "; ");
        log "current index: %d" state.index;
        state
    | None -> state
  else state

let process_rpc state rpc_fd cmd_fd =
  let client_fd, _ = Unix.accept rpc_fd in
  Fun.protect
    ~finally:(fun () -> Unix.close client_fd)
    (fun () ->
      let buf = Bytes.create 256 in
      let n = Unix.read client_fd buf 0 256 in
      let state, response, switch_to =
        match String.trim (Bytes.sub_string buf 0 n) with
        | "back" -> handle_back state
        | "forward" -> handle_forward state
        | _ -> (state, "unknown command", None)
      in
      Option.iter
        (fun ws ->
          log "go to: %s" (pp_workspace ws);
          send_i3_msg cmd_fd 0 (workspace_command ws);
          ignore (recv_i3_msg cmd_fd))
        switch_to;
      ignore
        (Unix.write_substring client_fd response 0 (String.length response));
      state)

let server sway rpc_endpoint =
  let socket_path = get_socket_path sway in
  let cmd_fd = connect_i3 socket_path in
  let evt_fd = connect_i3 socket_path in
  send_i3_msg evt_fd 2 {|["workspace"]|};
  ignore (recv_i3_msg evt_fd);
  (try Unix.unlink rpc_endpoint with Unix.Unix_error _ -> ());
  let rpc_fd = Unix.socket Unix.PF_UNIX Unix.SOCK_STREAM 0 in
  Unix.bind rpc_fd (Unix.ADDR_UNIX rpc_endpoint);
  Unix.listen rpc_fd 5;
  log "i3-workspace-history server started";
  let rec loop state =
    let readable, _, _ = Unix.select [ evt_fd; rpc_fd ] [] [] (-1.0) in
    readable
    |> List.fold_left
         (fun state fd ->
           if fd = evt_fd then (
             try process_i3_event state evt_fd
             with e ->
               log "error: %s" (Printexc.to_string e);
               state)
           else if fd = rpc_fd then (
             try process_rpc state rpc_fd cmd_fd
             with e ->
               log "error: %s" (Printexc.to_string e);
               state)
           else state)
         state
    |> loop
  in
  loop
    {
      jumplist = [];
      index = 0;
      next_ws = None;
      navigating = false;
      start_navigating = false;
    }

let client rpc_endpoint cmd =
  let fd = Unix.socket Unix.PF_UNIX Unix.SOCK_STREAM 0 in
  (try Unix.connect fd (Unix.ADDR_UNIX rpc_endpoint)
   with Unix.Unix_error (e, _, _) ->
     Printf.eprintf "failed: %s\n" (Unix.error_message e);
     exit 1);
  ignore (Unix.write_substring fd cmd 0 (String.length cmd));
  let buf = Bytes.create 256 in
  let n = Unix.read fd buf 0 256 in
  log "%s" (Bytes.sub_string buf 0 n);
  Unix.close fd

type mode = Server | Back | Forward

let () =
  let open Cmdliner in
  let sway =
    Arg.(value & flag & info [ "sway" ] ~doc:"Use sway instead of i3")
  in
  let mode =
    Arg.(
      value
      & opt
          (enum [ ("server", Server); ("back", Back); ("forward", Forward) ])
          Server
      & info [ "mode" ] ~doc:"Operation mode")
  in
  let run sway mode =
    let ep =
      if sway then "/tmp/swayjumplist-socket" else "/tmp/i3jumplist-socket"
    in
    match mode with
    | Server -> server sway ep
    | Back -> client ep "back"
    | Forward -> client ep "forward"
  in
  Cmd.v
    (Cmd.info "i3-workspace-history"
       ~doc:"Vim-like jumplist for i3/sway workspaces")
    Term.(const run $ sway $ mode)
  |> Cmd.eval |> exit
