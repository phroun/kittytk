"""KittyTK app-side client (Python port of the Go `client` package).

Typed handles with synchronous-looking reads served from an app-side
replica, writes as fire-and-forget protocol statements, and event
subscriptions folded into the replica before app handlers run - the same
veneer contract as the Go client, speaking the identical wire protocol to
the identical display service.
"""

from __future__ import annotations

import os
import queue
import socket
import tempfile
import threading
from typing import Callable, Dict, List, Optional

from . import protocol
from .protocol import Event, FlagState

DISPLAY_ENV = "KITTYTK_DISPLAY"


def default_socket_path() -> str:
    """The conventional endpoint: $KITTYTK_DISPLAY, else
    $XDG_RUNTIME_DIR/kittytk/display-0.sock."""
    p = os.environ.get(DISPLAY_ENV)
    if p:
        return p
    runtime = os.environ.get("XDG_RUNTIME_DIR") or tempfile.gettempdir()
    return os.path.join(runtime, "kittytk", "display-0.sock")


_CLOSED = object()  # reply-queue sentinel: the transport disconnected


class _ObjState:
    __slots__ = ("checked", "text", "selected", "result")

    def __init__(self):
        self.checked = FlagState.NONE
        self.text = ""
        self.selected = -1
        self.result = ""


class Conn:
    """One connection to one display service (never global: an app may
    hold any number of connections)."""

    def __init__(self, sock: socket.socket, dispatch: Optional[Callable[[str], None]]):
        self._sock = sock
        self._rfile = sock.makefile("rb")
        self._scanner = protocol.Scanner(self._rfile)
        self._dispatch = dispatch

        self._write_lock = threading.Lock()
        self._replies: "queue.Queue" = queue.Queue()
        self._events: "queue.Queue" = queue.Queue()

        self._lock = threading.Lock()
        self._state: Dict[int, _ObjState] = {}
        self._handlers: Dict[int, Dict[str, List[Callable[[Event], None]]]] = {}
        self._type_handlers: Dict[str, List[Callable[[Event], None]]] = {}
        self._subs = set()

        self._closed_flag = False
        self.closed = threading.Event()  # set when the connection ends

    # --- lifecycle -------------------------------------------------------

    def _start(self):
        threading.Thread(target=self._read_loop, daemon=True).start()
        threading.Thread(target=self._event_loop, daemon=True).start()

    def _read_loop(self):
        try:
            while True:
                text = self._scanner.next()
                try:
                    script = protocol.parse(text)
                except protocol.ParseError:
                    continue  # malformed inbound statement; skip
                for stmt in script.statements:
                    if stmt.verb == "reply":
                        try:
                            ids = protocol.decode_reply(stmt)
                            self._replies.put(("reply", ids))
                        except Exception as e:  # noqa: BLE001
                            self._replies.put(("error", str(e)))
                    elif stmt.verb == "error":
                        msg = "display error"
                        for a in stmt.args:
                            if a.name == "text" and a.value is not None \
                                    and a.value.kind == protocol.ValueKind.STRING:
                                msg = a.value.str
                        self._replies.put(("error", msg))
                    elif stmt.verb == "event":
                        try:
                            self._events.put(protocol.parse_event(text))
                        except Exception:  # noqa: BLE001
                            pass
        except (EOFError, OSError, ValueError):
            pass
        finally:
            self._mark_closed()

    def _event_loop(self):
        while True:
            ev = self._events.get()
            if ev is None:
                return
            self.deliver(ev)

    def _mark_closed(self):
        with self._lock:
            if self._closed_flag:
                return
            self._closed_flag = True
        try:
            self._sock.close()
        except OSError:
            pass
        self._replies.put(_CLOSED)   # unblock a waiting exec
        self._events.put(None)       # stop the event loop
        self.closed.set()

    def close(self):
        self._mark_closed()

    # --- request / reply -------------------------------------------------

    def exec(self, src: str) -> Dict[str, int]:
        """Execute one batch of protocol text; returns the surfaced
        name->id map, or raises on a display error / disconnect."""
        with self._write_lock:
            with self._lock:
                if self._closed_flag:
                    raise ConnectionError("connection closed")
            self._sock.sendall((src + "\nend\n").encode("utf-8"))
            item = self._replies.get()
            if item is _CLOSED:
                raise ConnectionError("connection closed")
            kind, payload = item
            if kind == "error":
                raise RuntimeError(payload)
            return payload

    def build(self, src: str) -> "UI":
        return UI(self, self.exec(src))

    # --- events & replica ------------------------------------------------

    def deliver(self, ev: Event):
        tid = ev.trinket() or 0
        with self._lock:
            st = self._state.get(tid)
            if st is None:
                st = _ObjState()
                self._state[tid] = st
            dispatch_action = None
            if ev.type == "toggle":
                st.checked = ev.flag("checked")
            elif ev.type == "change":
                s = ev.text("text")
                if s is not None:
                    st.text = s
                n = ev.int_("selected")
                if n is not None:
                    st.selected = n
            elif ev.type == "finish":
                w = ev.word("result")
                if w is not None:
                    st.result = w
            elif ev.type == "command":
                a = ev.word("action")
                if a is not None:
                    dispatch_action = a
            fns = list(self._handlers.get(tid, {}).get(ev.type, ()))
            fns.extend(self._type_handlers.get(ev.type, ()))
            dispatch = self._dispatch
        if dispatch_action and dispatch:
            dispatch(dispatch_action)
        for fn in fns:
            fn(ev)

    def _ensure_sub(self, oid: int, event: str):
        with self._lock:
            key = (oid, event)
            if key in self._subs:
                return
            self._subs.add(key)
        try:
            self.exec("sub %d %s" % (oid, event))
        except Exception:  # noqa: BLE001
            pass  # connection without event support; replica never updates

    def on(self, oid: int, event: str, fn: Callable[[Event], None]):
        self._ensure_sub(oid, event)
        with self._lock:
            self._handlers.setdefault(oid, {}).setdefault(event, []).append(fn)

    def on_command(self, action: str, fn: Callable[[], None]):
        def handler(ev: Event):
            if ev.word("action") == action:
                fn()
        with self._lock:
            self._type_handlers.setdefault("command", []).append(handler)

    def state_of(self, oid: int) -> _ObjState:
        with self._lock:
            st = self._state.get(oid)
            if st is None:
                st = _ObjState()
                self._state[oid] = st
            return st

    def set(self, oid: int, args: str):
        self.exec("set %d %s" % (oid, args))


# --- Handles -------------------------------------------------------------

class Handle:
    def __init__(self, conn: Conn, oid: int):
        self._c = conn
        self._id = oid

    @property
    def id(self) -> int:
        return self._id

    def valid(self) -> bool:
        return self._c is not None and self._id != 0

    def set(self, args: str):
        self._c.set(self._id, args)

    def destroy(self):
        self._c.exec("destroy %d" % self._id)

    def on(self, event: str, fn: Callable[[Event], None]):
        self._c.on(self._id, event, fn)


class Button(Handle):
    def on_click(self, fn: Callable[[], None]):
        self.on("click", lambda ev: fn())

    def set_caption(self, s: str):
        self.set("caption=" + protocol.quote(s))


class Label(Handle):
    def set_caption(self, s: str):
        self.set("caption=" + protocol.quote(s))


class Checkbox(Handle):
    def state(self) -> FlagState:
        s = self._c.state_of(self._id).checked
        return s if s != FlagState.NONE else FlagState.FALSE

    def checked(self) -> bool:
        return self.state() == FlagState.TRUE

    def set_checked(self, v: bool):
        st = self._c.state_of(self._id)
        if v:
            st.checked = FlagState.TRUE
            self.set("checked")
        else:
            st.checked = FlagState.FALSE
            self.set("!checked")

    def on_toggle(self, fn: Callable[[FlagState], None]):
        self.on("toggle", lambda ev: fn(ev.flag("checked")))


class TextInput(Handle):
    def text(self) -> str:
        return self._c.state_of(self._id).text

    def set_text(self, s: str):
        self._c.state_of(self._id).text = s
        self.set("text=" + protocol.quote(s))

    def on_change(self, fn: Callable[[str], None]):
        def handler(ev: Event):
            s = ev.text("text")
            if s is not None:
                fn(s)
        self.on("change", handler)


class Selector(Handle):
    def selected(self) -> int:
        return self._c.state_of(self._id).selected

    def select(self, index: int):
        self._c.state_of(self._id).selected = index
        self.set("selected=%d" % index)

    def on_change(self, fn: Callable[[int], None]):
        def handler(ev: Event):
            n = ev.int_("selected")
            if n is not None:
                fn(n)
        self.on("change", handler)


class Window(Handle):
    def on_closed(self, fn: Callable[[], None]):
        self.on("window_closed", lambda ev: fn())

    def close(self):
        self.destroy()

    def set_title(self, s: str):
        self.set("title=" + protocol.quote(s))


class UI:
    """Handle access to one build's surfaced names."""

    def __init__(self, conn: Conn, ids: Dict[str, int]):
        self._conn = conn
        self._ids = ids

    def id(self, name: str) -> int:
        return self._ids.get(name, 0)

    def has(self, name: str) -> bool:
        return name in self._ids

    def _handle(self, name: str, *mirrors: str):
        oid = self._ids.get(name, 0)
        if oid != 0:
            for ev in mirrors:
                self._conn._ensure_sub(oid, ev)
        return oid

    def object(self, name: str) -> Handle:
        return Handle(self._conn, self._handle(name))

    def button(self, name: str) -> Button:
        return Button(self._conn, self._handle(name))

    def label(self, name: str) -> Label:
        return Label(self._conn, self._handle(name))

    def checkbox(self, name: str) -> Checkbox:
        return Checkbox(self._conn, self._handle(name, "toggle"))

    def text_input(self, name: str) -> TextInput:
        return TextInput(self._conn, self._handle(name, "change"))

    def selector(self, name: str) -> Selector:
        return Selector(self._conn, self._handle(name, "change"))

    def window(self, name: str) -> Window:
        return Window(self._conn, self._handle(name))


# --- Dial ----------------------------------------------------------------

def _dial(path: str, app_name: str, dispatch, solo: bool) -> Conn:
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.connect(path)
    conn = Conn(sock, dispatch)

    hello = "hello version=1 app=" + protocol.quote(app_name)
    if solo:
        hello += " solo"
    sock.sendall((hello + "\nend\n").encode("utf-8"))

    welcome = conn._scanner.next()
    script = protocol.parse(welcome)
    if not script.statements or script.statements[0].verb != "welcome":
        sock.close()
        raise ConnectionError("handshake: unexpected response %r" % welcome)

    conn._start()
    return conn


def dial(path: str, app_name: str, dispatch=None) -> Conn:
    """Connect to a display service. dispatch (optional) receives action=
    command IDs."""
    return _dial(path, app_name, dispatch, False)


def dial_solo(path: str, app_name: str, dispatch=None) -> Conn:
    """dial() for an app that wants to be the whole display (its `main`
    window replaces the desktop)."""
    return _dial(path, app_name, dispatch, True)
