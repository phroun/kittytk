"""Interop smoke client: a Python app driving a REAL Go display host.

Run by python/interop_test.go, which stands up a headless KittyTK display
service and then drives input into the window this builds. It proves, over
an actual socket:

  * handshake + build (Python -> host): the surfaced ids come back,
  * write-through (Python -> host): SetText lands in the real trinket,
  * events (host -> Python): a server-side toggle and button click arrive
    as toggle/command events on this Python connection.

Protocol markers on stdout (the Go side reads them):
  READY        - built + subscribed; safe for the host to drive input
  TOGGLE ok    - received the toggle event
  COMMAND ok   - received the command event
  DONE         - both received; exiting 0
"""

import os
import sys
import threading

sys.path.insert(0, os.path.dirname(__file__))

import kittytk  # noqa: E402


def main(sock: str) -> int:
    conn = kittytk.dial(sock, "Py Interop App")

    ui = conn.build(
        'w=new window title="Py Interop" width=320 height=160 children={\n'
        '  p=new panel layout=vbox children={\n'
        '    cb=new checkbox caption="remote checkbox"\n'
        '    inp=new textinput\n'
        '    btn=new button caption="Go" action=remote.act\n'
        '  }\n'
        '}\n'
        'wcb=w.p.cb\n'
        'winp=w.p.inp\n'
        'wbtn=w.p.btn\n'
    )
    for key in ("w", "wcb", "winp", "wbtn"):
        if ui.id(key) == 0:
            print("FAIL missing id " + key, flush=True)
            return 1

    # App -> host write-through: the Go side reads this back from the real
    # trinket to prove the direction.
    ui.text_input("winp").set_text("over the wire")

    got_toggle = threading.Event()
    got_command = threading.Event()

    def on_toggle(state):
        # state is a FlagState; TRUE after the host toggles it on.
        print("TOGGLE ok state=%d" % int(state), flush=True)
        got_toggle.set()

    ui.checkbox("wcb").on_toggle(on_toggle)
    conn.on_command("remote.act", lambda: (print("COMMAND ok", flush=True), got_command.set()))

    print("READY", flush=True)  # host may now drive input

    if not got_toggle.wait(10) or not got_command.wait(10):
        print("TIMEOUT", flush=True)
        return 2

    print("DONE", flush=True)
    conn.close()
    return 0


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("usage: interop_smoke.py <socket>", file=sys.stderr)
        sys.exit(64)
    sys.exit(main(sys.argv[1]))
