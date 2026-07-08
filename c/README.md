# KittyTK — C client

A pure-C client for the KittyTK display protocol, plus the demo app. Like
the Python port, it proves the protocol is **language-neutral**: a C program
drives the exact same Go display host (`kittytk-tui` / `kittytk-sdl`) over
the identical wire. Depends only on libc + pthreads.

```
kittytk.h / kittytk.c   the client library: wire format (scanner, quoting,
                        flat inbound parser, events), socket transport,
                        reader + event pthreads, subscriptions, UI ids
scripts.h / scripts.c   the demo's protocol-text build scripts
demoapp.c               the interactive demo (menus, MDI, dialogs, secondary)
interop_smoke.c         bidirectional interop client (for the Go harness)
demoapp_smoke.c         full-demo build client (for the Go harness)
interop/                Go harness: a real headless host driving the C clients
```

## Build & run

```sh
make                                          # builds ./demoapp

# terminal 1 — a desktop host (either renderer):
go run ./examples/kittytk-tui                 # terminal
go run -tags sdl ./examples/kittytk-sdl       # graphical

# terminal 2 — the C app:
./demoapp                                     # attaches to the host
./demoapp --solo                              # becomes the whole display
```

## Verify

```sh
# Interop against a REAL Go host: the harness compiles the C smokes with cc,
# stands up a headless display service, and drives input into the C client,
# confirming events flow both ways over a live socket, plus the full demo
# build is accepted from C.
go test ./c/interop/
```
