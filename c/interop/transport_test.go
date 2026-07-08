package interop_c_test

// The same C interop exchange over tcp:// and tls:// instead of a unix
// socket. The tls:// case compiles the C client with -DKT_TLS and links
// OpenSSL, exercising mutual TLS + trust-on-first-use pinning end to end.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/phroun/kittytk/display"
	"github.com/phroun/kittytk/trinkets"
)

// buildCTLS compiles the smoke with TLS enabled (OpenSSL). It skips if
// cc or the OpenSSL toolchain isn't available.
func buildCTLS(t *testing.T, src string) string {
	t.Helper()
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skipf("cc not available: %v", err)
	}
	bin := filepath.Join(t.TempDir(), "smoke_tls")
	cmd := exec.Command("cc", "-std=c11", "-O2", "-DKT_TLS", "-o", bin,
		filepath.Join("..", src), filepath.Join("..", "kittytk.c"),
		filepath.Join("..", "scripts.c"), "-lpthread", "-lssl", "-lcrypto")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cc -DKT_TLS %s (OpenSSL unavailable?): %v\n%s", src, err, out)
	}
	return bin
}

func startServiceCfg(t *testing.T, cfg display.Config) (*trinkets.Desktop, string, func()) {
	t.Helper()
	desktop := trinkets.NewDesktop()
	desktop.SetBackend(&nullBackend{})

	ready := make(chan *display.Server, 1)
	errc := make(chan error, 1)
	desktop.SetOnStartup(func() {
		srv, err := display.ServeConfig(desktop, cfg)
		if err != nil {
			errc <- err
			desktop.Quit()
			return
		}
		ready <- srv
	})
	exited := make(chan int, 1)
	go func() { exited <- desktop.Run() }()

	select {
	case srv := <-ready:
		return desktop, srv.Addr(), func() {
			desktop.Quit()
			select {
			case <-exited:
			case <-time.After(5 * time.Second):
				t.Error("desktop did not exit")
			}
		}
	case err := <-errc:
		t.Fatalf("serve: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("desktop did not start")
	}
	return nil, "", func() {}
}

func TestCClientInteropTCP(t *testing.T) {
	bin := buildC(t, "interop_smoke.c")
	desktop, addr, stop := startServiceCfg(t, display.Config{Endpoint: "tcp://127.0.0.1:0"})
	defer stop()
	driveCInterop(t, desktop, bin, "tcp://"+addr, nil)
}

func TestCClientInteropTLS(t *testing.T) {
	bin := buildCTLS(t, "interop_smoke.c")

	dir := t.TempDir()
	t.Setenv(display.HostIdentityEnv, filepath.Join(dir, "host_identity.pem"))
	t.Setenv(display.AuthStoreEnv, filepath.Join(dir, "authorizations"))

	desktop, addr, stop := startServiceCfg(t, display.Config{Endpoint: "tls://127.0.0.1:0"})
	defer stop()

	cdir := t.TempDir()
	env := []string{
		"KITTYTK_IDENTITY=" + filepath.Join(cdir, "identity.pem"),
		"KITTYTK_KNOWN_HOSTS=" + filepath.Join(cdir, "known_hosts"),
	}
	driveCInterop(t, desktop, bin, "tls://"+addr, env)

	if b, err := os.ReadFile(filepath.Join(cdir, "known_hosts")); err != nil || len(b) == 0 {
		t.Errorf("C client did not pin the host in known_hosts (err=%v)", err)
	}
}
