/* demoapp.c - the KittyTK demo as a display-protocol application, in C.
 *
 * Port of examples/demoapp: it links only the kittytk C client, dials a
 * running display host (kittytk-tui / kittytk-sdl), and drives the whole
 * UI - the tabbed gallery, menus, MDI, dialogs, secondary apps - over the
 * socket.
 *
 *   # terminal 1: a desktop host
 *   go run ./examples/kittytk-tui             (or -tags sdl ./examples/kittytk-sdl)
 *   # terminal 2: this app
 *   ./demoapp            (or ./demoapp --solo to become the whole display)
 */
#define _POSIX_C_SOURCE 200809L
#include "kittytk.h"
#include "scripts.h"

#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

typedef struct {
    kt_conn *conn;
    const char *path;
    uint64_t win, tabs, mdi, mdistatus, mdidock;
    int mdi_count;
    int dock_seq;
    volatile int quit;
    pthread_mutex_t mu;
} App;

static void set_status(App *a, const char *text) {
    char *q = kt_quote(text);
    char *src = malloc(strlen(q) + 16);
    sprintf(src, "status text=%s", q);
    kt_exec(a->conn, src);
    free(src);
    free(q);
}

/* --- small callback closures (userdata carries what's needed) -------- */

static void on_window_closed(const kt_event *ev, void *ud) { (void)ev; ((App *)ud)->quit = 1; }

static void on_binput(const kt_event *ev, void *ud) {
    App *a = ud;
    const char *s = kt_event_text(ev, "text");
    if (s) { char buf[256]; snprintf(buf, sizeof buf, "Text: %s", s); set_status(a, buf); }
}

static void on_wfont(const kt_event *ev, void *ud) {
    App *a = ud;
    kt_set(a->conn, a->win, kt_event_flag(ev, "checked") == KT_FLAG_TRUE
           ? "font=\"tuesday12\"" : "font=\"default\"");
}
static void on_dfont(const kt_event *ev, void *ud) {
    App *a = ud;
    kt_exec(a->conn, kt_event_flag(ev, "checked") == KT_FLAG_TRUE
            ? "desktopfont tuesday" : "desktopfont default");
}
static void on_grid(const kt_event *ev, void *ud) {
    App *a = ud;
    kt_set(a->conn, a->win, kt_event_flag(ev, "checked") == KT_FLAG_TRUE
           ? "denomination=32" : "denomination=0");
}

/* background radios: userdata is the App; the arg is baked per-handler */
typedef struct { App *a; const char *arg; } BgCtx;
static void on_bg(const kt_event *ev, void *ud) {
    BgCtx *b = ud;
    if (kt_event_flag(ev, "checked") == KT_FLAG_TRUE) {
        char src[64];
        snprintf(src, sizeof src, "background=%s", b->arg);
        kt_set(b->a->conn, b->a->tabs, src);
    }
}

/* menu command handlers: bind a fixed verb via a small ctx */
typedef struct { App *a; const char *verb; } VerbCtx;
static void on_verb(void *ud) { VerbCtx *v = ud; kt_exec(v->a->conn, v->verb); }
typedef struct { App *a; const char *msg; } MsgCtx;
static void on_status_msg(void *ud) { MsgCtx *m = ud; set_status(m->a, m->msg); }

static void spawn_mdi_child(App *a);

typedef struct { App *a; } AppCtx;
static void on_mdi_spawn(void *ud) { spawn_mdi_child(((AppCtx *)ud)->a); }

static void on_mdi_set(void *ud) { VerbCtx *v = ud; char s[32]; snprintf(s, sizeof s, "%s", v->verb); kt_set(v->a->conn, v->a->mdi, s); }

static void show_about(void *ud) {
    App *a = ((AppCtx *)ud)->a;
    char *s = about_dialog_script();
    kt_exec(a->conn, s);
    free(s);
}

static void open_terminal_window(void *ud) {
    App *a = ((AppCtx *)ud)->a;
    pthread_mutex_lock(&a->mu);
    int n = ++a->mdi_count;
    pthread_mutex_unlock(&a->mu);
    char *s = demo_terminal_script(n);
    kt_ui *ui = kt_build(a->conn, s);
    free(s);
    if (ui) kt_ui_free(ui);  /* Close button wiring omitted in this port */
}

static void open_secondary(App *a);
static void on_new_window(void *ud) { open_secondary(((AppCtx *)ud)->a); }

/* MDI events */
static void on_mdi_active(const kt_event *ev, void *ud) {
    App *a = ud;
    const char *title = kt_event_text(ev, "title");
    char buf[160];
    snprintf(buf, sizeof buf, "Active: %s", (title && *title) ? title : "none");
    char *q = kt_quote(buf);
    char src[192];
    snprintf(src, sizeof src, "caption=%s", q);
    free(q);
    kt_set(a->conn, a->mdistatus, src);
}

/* MDI child New button spawns another document. */
static void on_mdi_child_new(const kt_event *ev, void *ud) { (void)ev; spawn_mdi_child(((AppCtx *)ud)->a); }

static void spawn_mdi_child(App *a) {
    pthread_mutex_lock(&a->mu);
    int n = ++a->mdi_count;
    pthread_mutex_unlock(&a->mu);
    char *s = mdi_child_script(n);
    kt_ui *ui = kt_build(a->conn, s);
    free(s);
    if (!ui) return;
    uint64_t nb = kt_ui_id(ui, "wnew");
    static AppCtx child_ctx;
    child_ctx.a = a;
    if (nb) kt_on(a->conn, nb, "click", on_mdi_child_new, &child_ctx);
    kt_ui_free(ui);
}

static void wire_protocol_window(App *a) {
    char *s = protocol_window_script();
    kt_ui *ui = kt_build(a->conn, s);
    free(s);
    if (!ui) return;
    kt_ui_free(ui);
}

static void open_secondary(App *a) {
    static int count = 0;
    int n = ++count;
    char name[32];
    snprintf(name, sizeof name, "App %d", n);
    kt_conn *c = kt_dial(a->path, name);
    if (!c) return;
    char *s = secondary_build_script(n);
    kt_ui *ui = kt_build(c, s);
    free(s);
    /* The secondary runs on its own connection until its window closes;
     * for this port we let it live for the process lifetime. */
    if (ui) kt_ui_free(ui);
}

/* command handler table entry */
typedef struct { const char *action; kt_command_cb cb; void *ud; } Cmd;

int main(int argc, char **argv) {
    int solo = (argc > 1 && strcmp(argv[1], "--solo") == 0);

    App a;
    memset(&a, 0, sizeof a);
    pthread_mutex_init(&a.mu, NULL);
    char *path = kt_default_socket_path();
    a.path = path;

    a.conn = solo ? kt_dial_solo(path, "KittyTK Demo") : kt_dial(path, "KittyTK Demo");
    if (!a.conn) {
        fprintf(stderr, "cannot reach display service at %s\n", path);
        fprintf(stderr, "start a desktop first: go run ./examples/kittytk-tui "
                        "(or -tags sdl ./examples/kittytk-sdl)\n");
        return 1;
    }

    char *main_src = main_build_script();
    kt_ui *ui = kt_build(a.conn, main_src);
    free(main_src);
    if (!ui) { fprintf(stderr, "build main window failed\n"); return 1; }

    a.win = kt_ui_id(ui, "w");
    a.tabs = kt_ui_id(ui, "tabs");
    a.mdi = kt_ui_id(ui, "mdi");
    a.mdistatus = kt_ui_id(ui, "mdistatus");
    a.mdidock = kt_ui_id(ui, "mdidock");

    /* interactive trinkets */
    kt_on(a.conn, kt_ui_id(ui, "binput"), "change", on_binput, &a);
    kt_on(a.conn, kt_ui_id(ui, "wfont"), "toggle", on_wfont, &a);
    kt_on(a.conn, kt_ui_id(ui, "dfont"), "toggle", on_dfont, &a);
    kt_on(a.conn, kt_ui_id(ui, "grid"), "toggle", on_grid, &a);

    static BgCtx bgdef, bggreen, bggray, sbgdef, sbggreen, sbggray;
    bgdef = (BgCtx){&a, "default"}; bggreen = (BgCtx){&a, "green"}; bggray = (BgCtx){&a, "\"#333333\""};
    sbgdef = (BgCtx){&a, "default"}; sbggreen = (BgCtx){&a, "green"}; sbggray = (BgCtx){&a, "\"#333333\""};
    kt_on(a.conn, kt_ui_id(ui, "bgdef"), "toggle", on_bg, &bgdef);
    kt_on(a.conn, kt_ui_id(ui, "bggreen"), "toggle", on_bg, &bggreen);
    kt_on(a.conn, kt_ui_id(ui, "bggray"), "toggle", on_bg, &bggray);
    kt_on(a.conn, kt_ui_id(ui, "sbgdef"), "toggle", on_bg, &sbgdef);
    kt_on(a.conn, kt_ui_id(ui, "sbggreen"), "toggle", on_bg, &sbggreen);
    kt_on(a.conn, kt_ui_id(ui, "sbggray"), "toggle", on_bg, &sbggray);

    /* menu commands (edit/view/window verbs go out as display app-verbs) */
    static AppCtx actx;
    actx.a = &a;
    static VerbCtx v_cut = {0}, v_copy = {0}, v_paste = {0}, v_sall = {0}, v_rawkey = {0},
                   v_theme = {0}, v_announce = {0}, v_speak = {0}, v_tile = {0}, v_cascade = {0};
#define VC(var, verbstr) var.a = &a; var.verb = verbstr;
    VC(v_cut, "cut") VC(v_copy, "copy") VC(v_paste, "paste") VC(v_sall, "selectall")
    VC(v_rawkey, "rawkey") VC(v_theme, "theme") VC(v_announce, "announce_visual")
    VC(v_speak, "announce_speak") VC(v_tile, "tile") VC(v_cascade, "cascade")

    kt_on_command(a.conn, "demo.file.new", open_terminal_window, &actx);
    kt_on_command(a.conn, "demo.edit.cut", on_verb, &v_cut);
    kt_on_command(a.conn, "demo.edit.copy", on_verb, &v_copy);
    kt_on_command(a.conn, "demo.edit.paste", on_verb, &v_paste);
    kt_on_command(a.conn, "demo.edit.selectall", on_verb, &v_sall);
    kt_on_command(a.conn, "demo.edit.rawkey", on_verb, &v_rawkey);
    kt_on_command(a.conn, "demo.view.theme", on_verb, &v_theme);
    kt_on_command(a.conn, "demo.view.announce", on_verb, &v_announce);
    kt_on_command(a.conn, "demo.view.speak", on_verb, &v_speak);
    kt_on_command(a.conn, "demo.window.new", on_new_window, &actx);
    kt_on_command(a.conn, "demo.window.tile", on_verb, &v_tile);
    kt_on_command(a.conn, "demo.window.cascade", on_verb, &v_cascade);
    kt_on_command(a.conn, "demo.help.about", show_about, &actx);

    static MsgCtx m_ok = {0}, m_cancel = {0}, m_apply = {0};
    m_ok.a = m_cancel.a = m_apply.a = &a;
    m_ok.msg = "OK button clicked!"; m_cancel.msg = "Cancel button clicked!"; m_apply.msg = "Apply button clicked!";
    kt_on_command(a.conn, "demo.basic.ok", on_status_msg, &m_ok);
    kt_on_command(a.conn, "demo.basic.cancel", on_status_msg, &m_cancel);
    kt_on_command(a.conn, "demo.basic.apply", on_status_msg, &m_apply);

    /* MDI */
    static VerbCtx mdi_tile = {0}, mdi_cascade = {0}, mdi_next = {0}, mdi_prev = {0};
    mdi_tile.a = mdi_cascade.a = mdi_next.a = mdi_prev.a = &a;
    mdi_tile.verb = "tile"; mdi_cascade.verb = "cascade"; mdi_next.verb = "next"; mdi_prev.verb = "prev";
    kt_on_command(a.conn, "demo.mdi.spawn", on_mdi_spawn, &actx);
    kt_on_command(a.conn, "demo.mdi.tile", on_mdi_set, &mdi_tile);
    kt_on_command(a.conn, "demo.mdi.cascade", on_mdi_set, &mdi_cascade);
    kt_on_command(a.conn, "demo.mdi.next", on_mdi_set, &mdi_next);
    kt_on_command(a.conn, "demo.mdi.prev", on_mdi_set, &mdi_prev);
    kt_on(a.conn, a.mdi, "active", on_mdi_active, &a);
    spawn_mdi_child(&a);

    wire_protocol_window(&a);

    /* end the demo when the main window closes (or the host disconnects) */
    kt_on(a.conn, a.win, "window_closed", on_window_closed, &a);
    kt_ui_free(ui);

    while (!a.quit && !kt_is_closed(a.conn)) {
        struct timespec ts = {0, 100 * 1000 * 1000};  /* 100ms */
        nanosleep(&ts, NULL);
    }

    kt_close(a.conn);
    free(path);
    return 0;
}
