package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/phroun/tuitk/app"
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
	"github.com/phroun/tuitk/widgets"
)

// The menu bars are protocol data (G6): scripts build menubar/menu/
// menuitem trees; activation dispatches action= command IDs through
// the application registry, where the app registers its handlers.

// buildMenuBar executes a menubar script and returns the menus plus
// the id->target table for reaching surfaced items (checkable state).
func buildMenuBar(script string) ([]*widgets.Menu, map[uint64]any, *protocol.Reply) {
	factory := &idCaptureFactory{
		inner: protocol.NewRegistryFactory(&protocol.BindContext{}),
		byID:  make(map[uint64]any),
	}
	parsed, err := protocol.Parse(script)
	if err != nil {
		panic(fmt.Sprintf("menu script: %v", err))
	}
	reply, err := protocol.NewSession().Execute(parsed, factory)
	if err != nil {
		panic(fmt.Sprintf("menu script: %v", err))
	}
	bar := factory.byID[reply.IDs["bar"]].(interface{ Menus() []*widgets.Menu })
	return bar.Menus(), factory.byID, reply
}

func mainMenuScript() string {
	var b strings.Builder
	b.WriteString(`
bar=new menubar children={
	new menu caption="&Demo" children={
		new menuitem caption="&New" shortcut="^N" action=demo.file.new
		new menuitem caption="&Open..." shortcut="^O"
		new menuitem caption="&Save" shortcut="^S"
	}
	new menu caption="&Edit" children={
		new menuitem caption="Cu&t" shortcut="^X"
		new menuitem caption="&Copy" shortcut="^C"
		new menuitem caption="&Paste" shortcut="^V"
		new menuitem separator
		new menuitem caption="&Raw Key Input" shortcut="^\\" action=demo.edit.rawkey
	}
	v=new menu caption="&View" children={
		new menuitem caption="&Toolbar" checkable checked
		new menuitem caption="&Status Bar" checkable checked
		new menuitem separator
		sr=new menuitem caption="Show A&nnouncements in Status Bar" checkable action=demo.view.announce
		speak=new menuitem caption="Speak Announcements (macOS)" checkable action=demo.view.speak`)
	if runtime.GOOS != "darwin" {
		b.WriteString(" !enabled")
	}
	b.WriteString(`
	}
	new menu caption="&Window" children={
		new menuitem caption="&New Window" action=demo.window.new
		new menuitem separator
		new menuitem caption="&Tile" action=demo.window.tile
		new menuitem caption="&Cascade" action=demo.window.cascade
	}
	new menu caption="&Alphabet" children={`)
	for i := 0; i < 26; i++ {
		letter := string(rune('A' + i))
		fmt.Fprintf(&b, "\n\t\tnew menuitem caption=\"&%s - Letter %s\"", letter, letter)
	}
	b.WriteString(`
	}
	new menu caption="&Help" children={
		new menuitem caption="&About" action=demo.help.about
	}
}
announce=bar.v.sr
speakitem=bar.v.speak
`)
	return b.String()
}

func createMenus(desktop *widgets.Desktop, application *app.Application) []*widgets.Menu {
	menus, _, _ := buildMenuBar(mainMenuScript())

	commands := application.Commands()
	commands.Register("demo.file.new", func() {
		application.AddWindow(createDemoWindow(desktop, application))
	})
	commands.Register("demo.edit.rawkey", func() {
		desktop.ActivatePassNextKeyToWidget()
	})
	commands.Register("demo.window.new", func() {
		newApp := createSecondaryApplication(desktop)
		desktop.AddApplication(newApp)
	})
	commands.Register("demo.window.tile", func() {
		desktop.WindowManager().TileWindows()
	})
	commands.Register("demo.window.cascade", func() {
		desktop.WindowManager().CascadeWindows()
	})
	commands.Register("demo.help.about", func() {
		showAboutDialog(desktop, application)
	})

	// Accessibility announcement routing. Replica discipline (slice
	// 4): the handlers OWN these booleans - the app's record of the
	// toggle intent - instead of reading the menu item's display-side
	// Checked state. Both flip on the same activation, so the check
	// mark and the behavior stay in step without a cross-seam read.
	var showVisualAnnouncements, speakAnnouncements bool
	var (
		speechMu  sync.Mutex
		speechCmd *exec.Cmd
	)
	updateAccessibilityHandler := func() {
		am := desktop.AccessibilityManager()
		if am == nil {
			return
		}
		if !showVisualAnnouncements && !speakAnnouncements {
			am.OnAnnounce = nil
			return
		}
		am.OnAnnounce = func(announcement core.AccessibilityAnnouncement) {
			if showVisualAnnouncements {
				if statusBar := desktop.StatusBar(); statusBar != nil {
					prefix := "📢"
					if announcement.Priority == "assertive" {
						prefix = "⚠️"
					}
					statusBar.SetText(fmt.Sprintf("%s [%s] %s", prefix, announcement.Priority, announcement.Message))
				}
			}
			if speakAnnouncements && runtime.GOOS == "darwin" {
				go func(msg string) {
					speechMu.Lock()
					if speechCmd != nil && speechCmd.Process != nil {
						_ = speechCmd.Process.Kill()
						_ = speechCmd.Wait()
					}
					speechCmd = exec.Command("say", "-r", "250", msg)
					speechMu.Unlock()
					_ = speechCmd.Run()
					speechMu.Lock()
					speechCmd = nil
					speechMu.Unlock()
				}(announcement.Message)
			}
		}
	}
	commands.Register("demo.view.announce", func() {
		showVisualAnnouncements = !showVisualAnnouncements
		updateAccessibilityHandler()
		if showVisualAnnouncements {
			if am := desktop.AccessibilityManager(); am != nil {
				am.AnnouncePolite("Visual announcements enabled")
			}
		}
	})
	commands.Register("demo.view.speak", func() {
		speakAnnouncements = !speakAnnouncements
		updateAccessibilityHandler()
		if speakAnnouncements {
			if am := desktop.AccessibilityManager(); am != nil {
				am.AnnouncePolite("Text to speech enabled")
			}
		}
	})

	return menus
}

func secondaryMenuScript(appNum int) string {
	return fmt.Sprintf(`
bar=new menubar children={
	new menu caption="&App %d" children={
		new menuitem caption="&Close Window" shortcut="^W" action=demo.app.close
	}
	new menu caption="&Edit" children={
		new menuitem caption="Cu&t" shortcut="^X"
		new menuitem caption="&Copy" shortcut="^C"
		new menuitem caption="&Paste" shortcut="^V"
		new menuitem separator
		new menuitem caption="&Raw Key Input" shortcut="^\\" action=demo.app.rawkey
	}
	new menu caption="&Info" children={
		new menuitem caption="&About This App" action=demo.app.info
	}
	new menu caption="&Help" children={
		new menuitem caption="&About" action=demo.app.about
	}
}
`, appNum)
}

func createSecondaryMenus(desktop *widgets.Desktop, application *app.Application, appNum int) []*widgets.Menu {
	menus, _, _ := buildMenuBar(secondaryMenuScript(appNum))

	// Each secondary application has its OWN registry, so the same
	// action IDs bind per-app without collision.
	commands := application.Commands()
	commands.Register("demo.app.close", func() {
		windows := application.Windows()
		if len(windows) > 0 {
			windows[0].Close()
		}
	})
	commands.Register("demo.app.rawkey", func() {
		desktop.ActivatePassNextKeyToWidget()
	})
	commands.Register("demo.app.info", func() {
		protocolMessageBox(application, fmt.Sprintf(
			`dlg=new messagebox icon=information ok title="About App %d" text="This is Secondary Application #%d\n\nIt has its own menus and status bar."`,
			appNum, appNum))
	})
	commands.Register("demo.app.about", func() {
		protocolMessageBox(application,
			`dlg=new messagebox icon=information ok title="About" text="Secondary Application\n\nDemonstrates multi-application support."`)
	})

	return menus
}
