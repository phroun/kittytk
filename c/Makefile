CC ?= cc
CFLAGS ?= -std=c11 -Wall -Wextra -O2
LDLIBS = -lpthread

.PHONY: all clean test
all: demoapp

demoapp: demoapp.c kittytk.c scripts.c kittytk.h scripts.h
	$(CC) $(CFLAGS) -o $@ demoapp.c kittytk.c scripts.c $(LDLIBS)

# Interop smoke binaries (also driven by the Go tests in c/interop).
interop_smoke: interop_smoke.c kittytk.c kittytk.h
	$(CC) $(CFLAGS) -o $@ interop_smoke.c kittytk.c $(LDLIBS)

demoapp_smoke: demoapp_smoke.c kittytk.c scripts.c kittytk.h scripts.h
	$(CC) $(CFLAGS) -o $@ demoapp_smoke.c kittytk.c scripts.c $(LDLIBS)

# Interop against a REAL Go host (bidirectional events + full demo build).
test:
	cd .. && go test ./c/interop/

clean:
	rm -f demoapp interop_smoke demoapp_smoke
