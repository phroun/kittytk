/* kittytk.h - the KittyTK display-protocol client, in C.
 *
 * A pure-protocol client (no rendering): it speaks the identical wire
 * language the Go and Python clients do, so a C program drives the same
 * display host (kittytk-tui / kittytk-sdl). Link with -lpthread.
 */
#ifndef KITTYTK_H
#define KITTYTK_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Flag state of a bare-name argument (values match the wire meaning). */
typedef enum {
    KT_FLAG_NONE = 0,   /* carries a value (name=value) */
    KT_FLAG_TRUE = 1,   /* bare name: `wrap` */
    KT_FLAG_FALSE = 2,  /* negated name: `!enabled` */
    KT_FLAG_INDET = 3   /* asserted-indeterminate: `?checked` */
} kt_flag;

typedef struct kt_conn kt_conn;
typedef struct kt_ui kt_ui;
typedef struct kt_event kt_event;

/* --- string quoting -------------------------------------------------- */

/* Quote s as a protocol string literal (quotes + escapes, control bytes
 * as \xNN). Returns a malloc'd string; caller frees. */
char *kt_quote(const char *s);

/* --- connection ------------------------------------------------------ */

/* The conventional endpoint ($KITTYTK_DISPLAY, else
 * $XDG_RUNTIME_DIR/kittytk/display-0.sock). malloc'd; caller frees. */
char *kt_default_socket_path(void);

/* Connect to a display service. Returns NULL on failure. dial_solo asks to
 * be the whole display (its `main` window replaces the desktop). */
kt_conn *kt_dial(const char *path, const char *app_name);
kt_conn *kt_dial_solo(const char *path, const char *app_name);

void kt_close(kt_conn *c);
int  kt_is_closed(kt_conn *c);       /* 1 once disconnected */
void kt_wait_closed(kt_conn *c);     /* block until the connection ends */

/* --- requests -------------------------------------------------------- */

/* Execute one batch of protocol text. Returns 0 on success, -1 on a
 * display error / disconnect. */
int kt_exec(kt_conn *c, const char *src);

/* Build a construction script; returns handle access to the surfaced
 * names (NULL on error). Free with kt_ui_free. */
kt_ui *kt_build(kt_conn *c, const char *src);
uint64_t kt_ui_id(const kt_ui *ui, const char *name);   /* 0 if absent */
void kt_ui_free(kt_ui *ui);

/* Property set / destroy on one object. */
int kt_set(kt_conn *c, uint64_t id, const char *args);
int kt_destroy(kt_conn *c, uint64_t id);

/* --- events ---------------------------------------------------------- */

const char *kt_event_type(const kt_event *ev);
/* Field readers: return 1 and fill *out when present with the right type. */
int kt_event_uint(const kt_event *ev, const char *name, uint64_t *out);
int kt_event_int(const kt_event *ev, const char *name, long long *out);
const char *kt_event_text(const kt_event *ev, const char *name); /* NULL if absent */
const char *kt_event_word(const kt_event *ev, const char *name); /* NULL if absent */
kt_flag kt_event_flag(const kt_event *ev, const char *name);
uint64_t kt_event_trinket(const kt_event *ev, int *ok);

typedef void (*kt_event_cb)(const kt_event *ev, void *userdata);
typedef void (*kt_command_cb)(void *userdata);

/* Subscribe to an event type from a specific object. */
void kt_on(kt_conn *c, uint64_t id, const char *event_type, kt_event_cb cb, void *ud);
/* Observe command events carrying the given action id. */
void kt_on_command(kt_conn *c, const char *action, kt_command_cb cb, void *ud);

#ifdef __cplusplus
}
#endif

#endif /* KITTYTK_H */
