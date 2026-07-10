package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/phroun/kittytk/app"
	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/protocol"
	"github.com/phroun/kittytk/style"
	"github.com/phroun/kittytk/trinkets"
)

// applyTerminalTheme walks a trinket subtree and puts every PurfecTerm
// into the given dark/light mode, so terminals follow the app theme
// toggle along with the rest of the UI.
func applyTerminalTheme(w core.Trinket, dark bool) {
	if w == nil {
		return
	}
	if term, ok := w.(*trinkets.PurfecTerm); ok {
		term.SetDarkTheme(dark)
	}
	if c, ok := w.(core.Container); ok {
		for _, child := range c.Children() {
			applyTerminalTheme(child, dark)
		}
	}
}

// The menu bars are protocol data (G6): scripts build menubar/menu/
// menuitem trees; activation dispatches action= command IDs through
// the application registry, where the app registers its handlers.

// buildMenuBar executes a menubar script and returns the menus plus
// the id->target table for reaching surfaced items (checkable state).
func buildMenuBar(script string) ([]*trinkets.Menu, map[uint64]any, *protocol.Reply) {
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
	bar := factory.byID[reply.IDs["bar"]].(interface{ Menus() []*trinkets.Menu })
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
	editmenu=new menu caption="&Edit" children={
		cutitem=new menuitem caption="Cu&t" shortcut="^X" action=demo.edit.cut
		new menuitem caption="&Copy" shortcut="^C" action=demo.edit.copy
		new menuitem caption="&Paste" shortcut="^V" action=demo.edit.paste
		new menuitem separator
		new menuitem caption="Select &All" action=demo.edit.selectall
		new menuitem separator
		new menuitem caption="&Raw Key Input" shortcut="^\\" action=demo.edit.rawkey
	}
	v=new menu caption="&View" children={
		new menuitem caption="&Toolbar" checkable checked
		new menuitem caption="&Status Bar" checkable checked
		new menuitem separator
		new menuitem caption="&Light/Dark Theme" shortcut="^T" action=demo.view.theme
		new menuitem separator
		sr=new menuitem caption="Show A&nnouncements in Status Bar" checkable action=demo.view.announce
		speak=new menuitem caption="Speak Announcements (macOS)" checkable action=demo.view.speak`)
	if runtime.GOOS != "darwin" {
		b.WriteString(" !enabled")
	}
	b.WriteString(`
	}
	new menu caption="&Window" wellknown="window" children={
		new menuitem caption="&New Window" action=demo.window.new
	}
	new menu caption="&Alphabet" children={`)
	for i := 0; i < 26; i++ {
		letter := string(rune('A' + i))
		fmt.Fprintf(&b, "\n\t\tnew menuitem caption=\"&%s - Letter %s\"", letter, letter)
		if i == 2 { // separator after "Letter C" (demo of the thin separator)
			b.WriteString("\n\t\tnew menuitem separator")
		}
	}
	b.WriteString(`
	}
	new menu caption="&Help" wellknown="help" children={
		new menuitem caption="&About" action=demo.help.about
	}
}
announce=bar.v.sr
speakitem=bar.v.speak
cutitem=bar.editmenu.cutitem
editmenu=bar.editmenu
`)
	return b.String()
}

func createMenus(desktop *trinkets.Desktop, application *app.Application) []*trinkets.Menu {
	menus, byID, reply := buildMenuBar(mainMenuScript())

	commands := application.Commands()
	commands.Register("demo.file.new", func() {
		application.AddWindow(createDemoWindow(desktop, application))
	})
	commands.Register("demo.edit.rawkey", func() {
		desktop.ActivatePassNextKeyToTrinket()
	})
	// Toggle the app between the dark and light theme palettes. The UI
	// chrome recolors on the next full-frame repaint (the 16 standard
	// colors read the active palette live); terminals follow via their
	// own dark/light palette.
	commands.Register("demo.view.theme", func() {
		newDark := style.ActiveTermTheme() == style.TermThemeLight
		if newDark {
			style.SetActiveTermTheme(style.TermThemeDark)
		} else {
			style.SetActiveTermTheme(style.TermThemeLight)
		}
		for _, win := range desktop.WindowManager().Windows() {
			applyTerminalTheme(win, newDark)
		}
		desktop.Update()
	})
	// Edit actions operate on the focused trinket; the context menus
	// on edit boxes invoke the same methods.
	type editActor interface {
		Cut()
		Copy()
		Paste()
		SelectAll()
	}
	editTarget := func() editActor {
		if fw := desktop.FocusedTrinket(); fw != nil {
			if ea, ok := fw.(editActor); ok {
				return ea
			}
		}
		return nil
	}
	commands.Register("demo.edit.cut", func() {
		if ea := editTarget(); ea != nil {
			ea.Cut()
		}
	})
	commands.Register("demo.edit.copy", func() {
		if ea := editTarget(); ea != nil {
			ea.Copy()
		}
	})
	commands.Register("demo.edit.paste", func() {
		if ea := editTarget(); ea != nil {
			ea.Paste()
		}
	})
	commands.Register("demo.edit.selectall", func() {
		if ea := editTarget(); ea != nil {
			ea.SelectAll()
		}
	})

	// Grey out Cut when the focused trinket reports it doesn't apply
	// (a terminal's output can't be cut). Refreshed each time the
	// Edit menu opens; the focused trinket is the previous active
	// window's while the menu is up.
	if em, ok := byID[reply.IDs["editmenu"]].(*trinkets.Menu); ok {
		cut, _ := byID[reply.IDs["cutitem"]].(*trinkets.MenuItem)
		em.SetOnAboutToShow(func() {
			if cut == nil {
				return
			}
			enabled := true
			if fw := desktop.FocusedTrinket(); fw != nil {
				if cq, ok := fw.(interface{ CutEnabled() bool }); ok {
					enabled = cq.CutEnabled()
				}
			}
			cut.SetEnabled(enabled)
		})
	}

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
			// Speech is throttled at the source (navigation announcements
			// mark themselves non-vocal while the user arrows quickly); the
			// status bar above still shows every one.
			if speakAnnouncements && announcement.Vocal && runtime.GOOS == "darwin" {
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
	editmenu=new menu caption="&Edit" children={
		cutitem=new menuitem caption="Cu&t" shortcut="^X" action=demo.app.cut
		new menuitem caption="&Copy" shortcut="^C" action=demo.app.copy
		new menuitem caption="&Paste" shortcut="^V" action=demo.app.paste
		new menuitem separator
		new menuitem caption="Select &All" action=demo.app.selectall
		new menuitem separator
		new menuitem caption="&Raw Key Input" shortcut="^\\" action=demo.app.rawkey
	}
	new menu caption="&Info" children={
		new menuitem caption="&About This App" action=demo.app.info
	}
	new menu caption="&Help" wellknown="help" children={
		new menuitem caption="&About" action=demo.app.about
	}
}
editmenu=bar.editmenu
cutitem=bar.editmenu.cutitem
`, appNum)
}

func createSecondaryMenus(desktop *trinkets.Desktop, application *app.Application, appNum int) []*trinkets.Menu {
	menus, byID, reply := buildMenuBar(secondaryMenuScript(appNum))

	// Each secondary application has its OWN registry, so the same
	// action IDs bind per-app without collision.
	commands := application.Commands()
	commands.Register("demo.app.close", func() {
		windows := application.Windows()
		if len(windows) > 0 {
			windows[0].Close()
		}
	})

	// Edit actions operate on the focused trinket, just like the main
	// menu; the context menus on edit boxes invoke the same methods.
	type editActor interface {
		Cut()
		Copy()
		Paste()
		SelectAll()
	}
	editTarget := func() editActor {
		if fw := desktop.FocusedTrinket(); fw != nil {
			if ea, ok := fw.(editActor); ok {
				return ea
			}
		}
		return nil
	}
	commands.Register("demo.app.cut", func() {
		if ea := editTarget(); ea != nil {
			ea.Cut()
		}
	})
	commands.Register("demo.app.copy", func() {
		if ea := editTarget(); ea != nil {
			ea.Copy()
		}
	})
	commands.Register("demo.app.paste", func() {
		if ea := editTarget(); ea != nil {
			ea.Paste()
		}
	})
	commands.Register("demo.app.selectall", func() {
		if ea := editTarget(); ea != nil {
			ea.SelectAll()
		}
	})

	// Grey out Cut when the focused trinket reports it doesn't apply
	// (a terminal's output can't be cut), matching the main Edit menu.
	if em, ok := byID[reply.IDs["editmenu"]].(*trinkets.Menu); ok {
		cut, _ := byID[reply.IDs["cutitem"]].(*trinkets.MenuItem)
		em.SetOnAboutToShow(func() {
			if cut == nil {
				return
			}
			enabled := true
			if fw := desktop.FocusedTrinket(); fw != nil {
				if cq, ok := fw.(interface{ CutEnabled() bool }); ok {
					enabled = cq.CutEnabled()
				}
			}
			cut.SetEnabled(enabled)
		})
	}

	commands.Register("demo.app.rawkey", func() {
		desktop.ActivatePassNextKeyToTrinket()
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
