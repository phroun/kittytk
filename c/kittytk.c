/* kittytk.c - implementation of the KittyTK display-protocol client in C.
 * A faithful port of the wire format and the client's read/event loops. */
#define _GNU_SOURCE
#include "kittytk.h"

#include <ctype.h>
#include <errno.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

/* --- growable byte buffer ------------------------------------------- */

typedef struct { char *p; size_t len, cap; } kt_buf;
static void buf_put(kt_buf *b, char c) {
    if (b->len + 1 >= b->cap) {
        b->cap = b->cap ? b->cap * 2 : 64;
        b->p = realloc(b->p, b->cap);
    }
    b->p[b->len++] = c;
}
static char *buf_dup(kt_buf *b) {
    char *s = malloc(b->len + 1);
    memcpy(s, b->p, b->len);
    s[b->len] = '\0';
    return s;
}

/* --- string quoting -------------------------------------------------- */

char *kt_quote(const char *s) {
    kt_buf b = {0};
    buf_put(&b, '"');
    for (; *s; s++) {
        unsigned char c = (unsigned char)*s;
        if (c == '"') { buf_put(&b, '\\'); buf_put(&b, '"'); }
        else if (c == '\\') { buf_put(&b, '\\'); buf_put(&b, '\\'); }
        else if (c == '\n') { buf_put(&b, '\\'); buf_put(&b, 'n'); }
        else if (c == '\t') { buf_put(&b, '\\'); buf_put(&b, 't'); }
        else if (c == '\r') { buf_put(&b, '\\'); buf_put(&b, 'r'); }
        else if (c == 0x1b) { buf_put(&b, '\\'); buf_put(&b, 'e'); }
        else if (c < 0x20 || c == 0x7f) {
            char tmp[5];
            snprintf(tmp, sizeof tmp, "\\x%02x", c);
            for (char *t = tmp; *t; t++) buf_put(&b, *t);
        } else buf_put(&b, (char)c);
    }
    buf_put(&b, '"');
    return buf_dup(&b);
}

char *kt_default_socket_path(void) {
    const char *env = getenv("KITTYTK_DISPLAY");
    if (env && *env) return strdup(env);
    const char *rt = getenv("XDG_RUNTIME_DIR");
    if (!rt || !*rt) rt = "/tmp";
    size_t n = strlen(rt) + strlen("/kittytk/display-0.sock") + 1;
    char *out = malloc(n);
    snprintf(out, n, "%s/kittytk/display-0.sock", rt);
    return out;
}

/* --- parsed statement ------------------------------------------------ */

typedef struct {
    char *name;
    kt_flag flag;    /* KT_FLAG_NONE when it has a value */
    int has_value;
    int kind;        /* 0=int 1=float 2=string 3=word */
    long long ival;
    double fval;
    char *sval;      /* string (unescaped) or word text */
} kt_arg;

typedef struct { char *verb; kt_arg *args; int n; } kt_stmt;

struct kt_event { const char *type; const kt_arg *fields; int n; };

static void stmt_free(kt_stmt *s) {
    if (!s) return;
    free(s->verb);
    for (int i = 0; i < s->n; i++) { free(s->args[i].name); free(s->args[i].sval); }
    free(s->args);
    free(s);
}

/* Flat parser: inbound statements (welcome/reply/error/event) are always
 * `verb arg...` with no nested blocks, so this covers the whole inbound
 * grammar. */
typedef struct { const char *s; size_t pos, len; } kt_p;

static int p_eof(kt_p *p) { return p->pos >= p->len; }
static char p_peek(kt_p *p) { return p_eof(p) ? '\0' : p->s[p->pos]; }
static void p_skip_inline(kt_p *p) {
    while (!p_eof(p)) {
        char c = p_peek(p);
        if (c == ' ' || c == '\t' || c == '\r' || c == '\n') p->pos++;
        else break;
    }
}
static int is_word_start(char c) { return c == '_' || isalpha((unsigned char)c); }
static int is_word_rune(char c) { return is_word_start(c) || c == '.' || isdigit((unsigned char)c); }

static char *p_word(kt_p *p) {
    kt_buf b = {0};
    while (!p_eof(p) && is_word_rune(p_peek(p))) buf_put(&b, p->s[p->pos++]);
    char *w = buf_dup(&b);
    free(b.p);
    return w;
}

static int hexv(char c) {
    if (c >= '0' && c <= '9') return c - '0';
    if (c >= 'a' && c <= 'f') return c - 'a' + 10;
    if (c >= 'A' && c <= 'F') return c - 'A' + 10;
    return -1;
}

static char *p_string(kt_p *p) {  /* assumes current char is '"' */
    kt_buf b = {0};
    p->pos++;  /* opening quote */
    while (!p_eof(p)) {
        char c = p->s[p->pos++];
        if (c == '"') break;
        if (c == '\\' && !p_eof(p)) {
            char e = p->s[p->pos++];
            switch (e) {
            case '\\': buf_put(&b, '\\'); break;
            case '"': buf_put(&b, '"'); break;
            case 'n': buf_put(&b, '\n'); break;
            case 't': buf_put(&b, '\t'); break;
            case 'r': buf_put(&b, '\r'); break;
            case 'e': buf_put(&b, 0x1b); break;
            case 'x': {
                int hi = p_eof(p) ? -1 : hexv(p->s[p->pos++]);
                int lo = p_eof(p) ? -1 : hexv(p->s[p->pos++]);
                if (hi >= 0 && lo >= 0) buf_put(&b, (char)(hi << 4 | lo));
                break;
            }
            default: buf_put(&b, e); break;
            }
        } else buf_put(&b, c);
    }
    char *s = buf_dup(&b);
    free(b.p);
    return s;
}

static void p_value(kt_p *p, kt_arg *a) {
    char c = p_peek(p);
    if (c == '"') {
        a->kind = 2; a->has_value = 1; a->sval = p_string(p);
    } else if (c == '-' || isdigit((unsigned char)c)) {
        kt_buf b = {0};
        int dot = 0;
        if (c == '-') buf_put(&b, p->s[p->pos++]);
        while (!p_eof(p)) {
            char d = p_peek(p);
            if (isdigit((unsigned char)d)) buf_put(&b, p->s[p->pos++]);
            else if (d == '.' && !dot) { dot = 1; buf_put(&b, p->s[p->pos++]); }
            else break;
        }
        char *num = buf_dup(&b); free(b.p);
        a->has_value = 1;
        if (dot) { a->kind = 1; a->fval = strtod(num, NULL); }
        else { a->kind = 0; a->ival = strtoll(num, NULL, 10); }
        free(num);
    } else {
        a->kind = 3; a->has_value = 1; a->sval = p_word(p);
    }
}

static kt_stmt *parse_statement(const char *text) {
    kt_p p = {text, 0, strlen(text)};
    p_skip_inline(&p);
    if (p_eof(&p) || !is_word_start(p_peek(&p))) return NULL;
    kt_stmt *st = calloc(1, sizeof *st);
    st->verb = p_word(&p);
    int cap = 0;
    for (;;) {
        p_skip_inline(&p);
        if (p_eof(&p) || p_peek(&p) == ';') break;
        char c = p_peek(&p);
        kt_arg a;
        memset(&a, 0, sizeof a);
        if (c == '!' || c == '?') {
            p.pos++;
            a.name = p_word(&p);
            a.flag = (c == '?') ? KT_FLAG_INDET : KT_FLAG_FALSE;
        } else if (is_word_start(c)) {
            a.name = p_word(&p);
            p_skip_inline(&p);
            if (!p_eof(&p) && p_peek(&p) == '=') { p.pos++; p_value(&p, &a); a.flag = KT_FLAG_NONE; }
            else a.flag = KT_FLAG_TRUE;
        } else if (c == '-' || isdigit((unsigned char)c)) {
            p_value(&p, &a);  /* bare number (target ref) */
        } else break;
        if (st->n + 1 > cap) { cap = cap ? cap * 2 : 8; st->args = realloc(st->args, cap * sizeof(kt_arg)); }
        st->args[st->n++] = a;
    }
    return st;
}

/* --- event field readers -------------------------------------------- */

static const kt_arg *ev_field(const kt_event *ev, const char *name) {
    for (int i = 0; i < ev->n; i++)
        if (ev->fields[i].name && strcmp(ev->fields[i].name, name) == 0) return &ev->fields[i];
    return NULL;
}
const char *kt_event_type(const kt_event *ev) { return ev->type; }
int kt_event_uint(const kt_event *ev, const char *name, uint64_t *out) {
    const kt_arg *a = ev_field(ev, name);
    if (!a || !a->has_value || a->kind != 0 || a->ival < 0) return 0;
    *out = (uint64_t)a->ival; return 1;
}
int kt_event_int(const kt_event *ev, const char *name, long long *out) {
    const kt_arg *a = ev_field(ev, name);
    if (!a || !a->has_value || a->kind != 0) return 0;
    *out = a->ival; return 1;
}
const char *kt_event_text(const kt_event *ev, const char *name) {
    const kt_arg *a = ev_field(ev, name);
    return (a && a->has_value && a->kind == 2) ? a->sval : NULL;
}
const char *kt_event_word(const kt_event *ev, const char *name) {
    const kt_arg *a = ev_field(ev, name);
    return (a && a->has_value && a->kind == 3) ? a->sval : NULL;
}
kt_flag kt_event_flag(const kt_event *ev, const char *name) {
    const kt_arg *a = ev_field(ev, name);
    if (!a || a->has_value) return KT_FLAG_NONE;
    return a->flag;
}
uint64_t kt_event_trinket(const kt_event *ev, int *ok) {
    uint64_t v;
    if (kt_event_uint(ev, "trinket", &v)) { if (ok) *ok = 1; return v; }
    if (kt_event_uint(ev, "window", &v)) { if (ok) *ok = 1; return v; }
    if (ok) *ok = 0;
    return 0;
}

/* --- connection ------------------------------------------------------ */

typedef struct { char *name; uint64_t id; } kt_pair;
struct kt_ui { kt_pair *pairs; int n; };

typedef struct evnode { char *text; struct evnode *next; } evnode;

typedef struct {
    uint64_t id;         /* 0 for command-only handlers */
    char *event_type;    /* NULL for command handlers */
    char *action;        /* non-NULL for command handlers */
    kt_event_cb cb;
    kt_command_cb ccb;
    void *ud;
} kt_handler;

struct kt_conn {
    int fd;
    unsigned char rbuf[4096];
    size_t rpos, rlen;
    int reof;

    pthread_mutex_t write_mu;

    pthread_mutex_t rmu; pthread_cond_t rcv;
    int reply_ready, reply_err;
    kt_ui reply_ids;
    char reply_errmsg[256];

    pthread_mutex_t emu; pthread_cond_t ecv;
    evnode *ehead, *etail;
    int estop;

    pthread_mutex_t hmu;
    kt_handler *handlers; int nh, caph;
    kt_pair *subs; int nsubs, capsubs;  /* (id, hash-of-type) sent */
    char **subtypes; /* parallel to subs: the type string */

    int closed;
    pthread_t rthread, ethread;
};

/* scanner: read one byte from the buffered fd, -1 at EOF/error. */
static int read_byte(kt_conn *c) {
    if (c->rpos >= c->rlen) {
        if (c->reof) return -1;
        ssize_t n = read(c->fd, c->rbuf, sizeof c->rbuf);
        if (n <= 0) { c->reof = 1; return -1; }
        c->rpos = 0; c->rlen = (size_t)n;
    }
    return c->rbuf[c->rpos++];
}

/* Frame the next statement (mirror of the Go Scanner). Returns malloc'd
 * text or NULL at EOF. */
static char *scan_next(kt_conn *c) {
    kt_buf b = {0};
    int depth = 0, in_string = 0, escaped = 0, saw = 0;
    for (;;) {
        int ch = read_byte(c);
        if (ch < 0) {
            char *r = (saw && depth == 0 && !in_string) ? buf_dup(&b) : NULL;
            free(b.p);
            return r;
        }
        if (escaped) escaped = 0;
        else if (in_string) {
            if (ch == '\\') escaped = 1;
            else if (ch == '"') in_string = 0;
        } else if (ch == '"') { in_string = 1; saw = 1; }
        else if (ch == '{') { depth++; saw = 1; }
        else if (ch == '}') { depth--; saw = 1; }
        else if (ch == '#') {
            for (;;) { int x = read_byte(c); if (x < 0 || x == '\n') break; }
            if (saw && depth == 0) { buf_put(&b, '\n'); char *r = buf_dup(&b); free(b.p); return r; }
            continue;
        } else if (ch == '\n') {
            if (depth == 0) {
                if (saw) { buf_put(&b, '\n'); char *r = buf_dup(&b); free(b.p); return r; }
                continue;
            }
        } else if (ch != ' ' && ch != '\t' && ch != '\r' && ch != ';') saw = 1;
        buf_put(&b, (char)ch);
    }
}

static void enqueue_event(kt_conn *c, const char *text) {
    evnode *n = malloc(sizeof *n);
    n->text = strdup(text);
    n->next = NULL;
    pthread_mutex_lock(&c->emu);
    if (c->etail) c->etail->next = n; else c->ehead = n;
    c->etail = n;
    pthread_cond_signal(&c->ecv);
    pthread_mutex_unlock(&c->emu);
}

static void dispatch_event(kt_conn *c, kt_stmt *st) {
    /* build a kt_event view: type = args[0].name, fields = args[1..] */
    if (st->n < 1) return;
    kt_event ev = { st->args[0].name, st->n > 1 ? &st->args[1] : NULL, st->n - 1 };
    int ok = 0;
    uint64_t tid = kt_event_trinket(&ev, &ok);
    const char *action = (strcmp(ev.type, "command") == 0) ? kt_event_word(&ev, "action") : NULL;

    /* snapshot matching handlers under the lock, then call outside it */
    pthread_mutex_lock(&c->hmu);
    kt_handler *snap = malloc(sizeof(kt_handler) * (c->nh ? c->nh : 1));
    int m = 0;
    for (int i = 0; i < c->nh; i++) {
        kt_handler *h = &c->handlers[i];
        if (h->action) {
            if (action && strcmp(h->action, action) == 0) snap[m++] = *h;
        } else if (ok && h->id == tid && h->event_type && strcmp(h->event_type, ev.type) == 0) {
            snap[m++] = *h;
        }
    }
    pthread_mutex_unlock(&c->hmu);

    for (int i = 0; i < m; i++) {
        if (snap[i].action) snap[i].ccb(snap[i].ud);
        else snap[i].cb(&ev, snap[i].ud);
    }
    free(snap);
}

static void *event_loop(void *arg) {
    kt_conn *c = arg;
    for (;;) {
        pthread_mutex_lock(&c->emu);
        while (!c->ehead && !c->estop) pthread_cond_wait(&c->ecv, &c->emu);
        if (!c->ehead && c->estop) { pthread_mutex_unlock(&c->emu); return NULL; }
        evnode *n = c->ehead;
        c->ehead = n->next;
        if (!c->ehead) c->etail = NULL;
        pthread_mutex_unlock(&c->emu);

        kt_stmt *st = parse_statement(n->text);
        if (st) { dispatch_event(c, st); stmt_free(st); }
        free(n->text);
        free(n);
    }
}

static void mark_closed(kt_conn *c) {
    pthread_mutex_lock(&c->rmu);
    c->closed = 1;
    c->reply_ready = 1;   /* unblock a waiting exec */
    c->reply_err = 1;
    snprintf(c->reply_errmsg, sizeof c->reply_errmsg, "connection closed");
    pthread_cond_broadcast(&c->rcv);
    pthread_mutex_unlock(&c->rmu);

    pthread_mutex_lock(&c->emu);
    c->estop = 1;
    pthread_cond_signal(&c->ecv);
    pthread_mutex_unlock(&c->emu);
}

static void *read_loop(void *arg) {
    kt_conn *c = arg;
    for (;;) {
        char *text = scan_next(c);
        if (!text) break;
        kt_stmt *st = parse_statement(text);
        if (!st) { free(text); continue; }
        if (strcmp(st->verb, "reply") == 0) {
            pthread_mutex_lock(&c->rmu);
            free(c->reply_ids.pairs); c->reply_ids.pairs = NULL; c->reply_ids.n = 0;
            for (int i = 0; i < st->n; i++) {
                if (st->args[i].has_value && st->args[i].kind == 0) {
                    c->reply_ids.pairs = realloc(c->reply_ids.pairs, (c->reply_ids.n + 1) * sizeof(kt_pair));
                    c->reply_ids.pairs[c->reply_ids.n].name = strdup(st->args[i].name);
                    c->reply_ids.pairs[c->reply_ids.n].id = (uint64_t)st->args[i].ival;
                    c->reply_ids.n++;
                }
            }
            c->reply_err = 0;
            c->reply_ready = 1;
            pthread_cond_signal(&c->rcv);
            pthread_mutex_unlock(&c->rmu);
        } else if (strcmp(st->verb, "error") == 0) {
            const char *msg = "display error";
            for (int i = 0; i < st->n; i++)
                if (strcmp(st->args[i].name ? st->args[i].name : "", "text") == 0
                    && st->args[i].has_value && st->args[i].kind == 2)
                    msg = st->args[i].sval;
            pthread_mutex_lock(&c->rmu);
            c->reply_err = 1;
            snprintf(c->reply_errmsg, sizeof c->reply_errmsg, "%s", msg);
            c->reply_ready = 1;
            pthread_cond_signal(&c->rcv);
            pthread_mutex_unlock(&c->rmu);
        } else if (strcmp(st->verb, "event") == 0) {
            enqueue_event(c, text);
        }
        stmt_free(st);
        free(text);
    }
    mark_closed(c);
    return NULL;
}

/* Write src + "\nend\n"; wait for the reply. ids may be NULL. */
static int do_exec(kt_conn *c, const char *src, kt_ui *out_ids) {
    pthread_mutex_lock(&c->write_mu);
    pthread_mutex_lock(&c->rmu);
    if (c->closed) { pthread_mutex_unlock(&c->rmu); pthread_mutex_unlock(&c->write_mu); return -1; }
    c->reply_ready = 0;
    pthread_mutex_unlock(&c->rmu);

    size_t n = strlen(src);
    /* write src, then the D22 end terminator */
    if (write(c->fd, src, n) < 0 || write(c->fd, "\nend\n", 5) < 0) {
        pthread_mutex_unlock(&c->write_mu);
        return -1;
    }

    pthread_mutex_lock(&c->rmu);
    while (!c->reply_ready) pthread_cond_wait(&c->rcv, &c->rmu);
    int err = c->reply_err;
    if (!err && out_ids) {
        out_ids->n = c->reply_ids.n;
        out_ids->pairs = malloc(sizeof(kt_pair) * (c->reply_ids.n ? c->reply_ids.n : 1));
        for (int i = 0; i < c->reply_ids.n; i++) {
            out_ids->pairs[i].name = strdup(c->reply_ids.pairs[i].name);
            out_ids->pairs[i].id = c->reply_ids.pairs[i].id;
        }
    }
    pthread_mutex_unlock(&c->rmu);
    pthread_mutex_unlock(&c->write_mu);
    return err ? -1 : 0;
}

int kt_exec(kt_conn *c, const char *src) { return do_exec(c, src, NULL); }

kt_ui *kt_build(kt_conn *c, const char *src) {
    kt_ui *ui = calloc(1, sizeof *ui);
    if (do_exec(c, src, ui) != 0) { free(ui->pairs); free(ui); return NULL; }
    return ui;
}
uint64_t kt_ui_id(const kt_ui *ui, const char *name) {
    if (!ui) return 0;
    for (int i = 0; i < ui->n; i++)
        if (strcmp(ui->pairs[i].name, name) == 0) return ui->pairs[i].id;
    return 0;
}
void kt_ui_free(kt_ui *ui) {
    if (!ui) return;
    for (int i = 0; i < ui->n; i++) free(ui->pairs[i].name);
    free(ui->pairs);
    free(ui);
}

int kt_set(kt_conn *c, uint64_t id, const char *args) {
    char *src = malloc(strlen(args) + 32);
    sprintf(src, "set %llu %s", (unsigned long long)id, args);
    int r = kt_exec(c, src);
    free(src);
    return r;
}
int kt_destroy(kt_conn *c, uint64_t id) {
    char src[32];
    snprintf(src, sizeof src, "destroy %llu", (unsigned long long)id);
    return kt_exec(c, src);
}

/* --- subscriptions & handlers --------------------------------------- */

static void ensure_sub(kt_conn *c, uint64_t id, const char *event) {
    pthread_mutex_lock(&c->hmu);
    for (int i = 0; i < c->nsubs; i++)
        if (c->subs[i].id == id && strcmp(c->subtypes[i], event) == 0) {
            pthread_mutex_unlock(&c->hmu);
            return;
        }
    if (c->nsubs + 1 > c->capsubs) {
        c->capsubs = c->capsubs ? c->capsubs * 2 : 8;
        c->subs = realloc(c->subs, c->capsubs * sizeof(kt_pair));
        c->subtypes = realloc(c->subtypes, c->capsubs * sizeof(char *));
    }
    c->subs[c->nsubs].id = id;
    c->subtypes[c->nsubs] = strdup(event);
    c->nsubs++;
    pthread_mutex_unlock(&c->hmu);

    char src[64];
    snprintf(src, sizeof src, "sub %llu %s", (unsigned long long)id, event);
    kt_exec(c, src);
}

static void add_handler(kt_conn *c, kt_handler h) {
    pthread_mutex_lock(&c->hmu);
    if (c->nh + 1 > c->caph) {
        c->caph = c->caph ? c->caph * 2 : 8;
        c->handlers = realloc(c->handlers, c->caph * sizeof(kt_handler));
    }
    c->handlers[c->nh++] = h;
    pthread_mutex_unlock(&c->hmu);
}

void kt_on(kt_conn *c, uint64_t id, const char *event_type, kt_event_cb cb, void *ud) {
    ensure_sub(c, id, event_type);
    kt_handler h = {0};
    h.id = id; h.event_type = strdup(event_type); h.cb = cb; h.ud = ud;
    add_handler(c, h);
}
void kt_on_command(kt_conn *c, const char *action, kt_command_cb cb, void *ud) {
    kt_handler h = {0};
    h.action = strdup(action); h.ccb = cb; h.ud = ud;
    add_handler(c, h);
}

/* --- dial / close ---------------------------------------------------- */

static kt_conn *dial(const char *path, const char *app_name, int solo) {
    int fd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (fd < 0) return NULL;
    struct sockaddr_un addr;
    memset(&addr, 0, sizeof addr);
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, path, sizeof addr.sun_path - 1);
    if (connect(fd, (struct sockaddr *)&addr, sizeof addr) < 0) { close(fd); return NULL; }

    kt_conn *c = calloc(1, sizeof *c);
    c->fd = fd;
    pthread_mutex_init(&c->write_mu, NULL);
    pthread_mutex_init(&c->rmu, NULL); pthread_cond_init(&c->rcv, NULL);
    pthread_mutex_init(&c->emu, NULL); pthread_cond_init(&c->ecv, NULL);
    pthread_mutex_init(&c->hmu, NULL);

    char *q = kt_quote(app_name);
    kt_buf hb = {0};
    const char *pre = "hello version=1 app=";
    for (const char *s = pre; *s; s++) buf_put(&hb, *s);
    for (char *s = q; *s; s++) buf_put(&hb, *s);
    free(q);
    if (solo) for (const char *s = " solo"; *s; s++) buf_put(&hb, *s);
    for (const char *s = "\nend\n"; *s; s++) buf_put(&hb, *s);
    if (write(fd, hb.p, hb.len) < 0) { free(hb.p); close(fd); free(c); return NULL; }
    free(hb.p);

    char *welcome = scan_next(c);
    if (!welcome) { close(fd); free(c); return NULL; }
    kt_stmt *st = parse_statement(welcome);
    int ok = st && strcmp(st->verb, "welcome") == 0;
    stmt_free(st);
    free(welcome);
    if (!ok) { close(fd); free(c); return NULL; }

    pthread_create(&c->rthread, NULL, read_loop, c);
    pthread_create(&c->ethread, NULL, event_loop, c);
    return c;
}

kt_conn *kt_dial(const char *path, const char *app_name) { return dial(path, app_name, 0); }
kt_conn *kt_dial_solo(const char *path, const char *app_name) { return dial(path, app_name, 1); }

int kt_is_closed(kt_conn *c) {
    pthread_mutex_lock(&c->rmu);
    int r = c->closed;
    pthread_mutex_unlock(&c->rmu);
    return r;
}
void kt_wait_closed(kt_conn *c) {
    pthread_join(c->rthread, NULL);
}
void kt_close(kt_conn *c) {
    if (!c) return;
    shutdown(c->fd, SHUT_RDWR);
    close(c->fd);
    pthread_join(c->rthread, NULL);
    pthread_join(c->ethread, NULL);
    /* (leak-free teardown of handler/sub tables omitted for brevity;
     * process exit reclaims them in the demo/smoke usage.) */
    free(c);
}
