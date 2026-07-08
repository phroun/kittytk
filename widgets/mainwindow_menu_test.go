package widgets

import (
	"strings"
	"testing"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/window"
)

// mockApp is a minimal ApplicationProvider for exercising the desktop's
// menu-bar composition.
type mockApp struct {
	name  string
	main  *window.Window
	menus []*Menu
}

func (a *mockApp) Name() string                      { return a.name }
func (a *mockApp) Windows() []*window.Window         { return nil }
func (a *mockApp) MainWindow() *window.Window        { return a.main }
func (a *mockApp) AddWindow(*window.Window)          {}
func (a *mockApp) RemoveWindow(*window.Window)       {}
func (a *mockApp) MenuBarContent() []*Menu           { return a.menus }
func (a *mockApp) StatusBarContent() []StatusSection { return nil }
func (a *mockApp) OnActivate()                       {}
func (a *mockApp) OnDeactivate()                     {}
func (a *mockApp) SetDesktop(core.Widget)            {}
func (a *mockApp) PassNextKeyToWidget() bool         { return false }
func (a *mockApp) ActivatePassNextKeyToWidget()      {}
func (a *mockApp) ClearPassNextKeyToWidget()         {}

func menuTitles(mb *MenuBar) []string {
	var out []string
	for _, m := range mb.Menus() {
		out = append(out, m.Title())
	}
	return out
}

func menuHasItem(m *Menu, substr string) bool {
	for _, it := range m.Items() {
		if strings.Contains(it.Text, substr) {
			return true
		}
	}
	return false
}

func TestDesktopMenuBarFullVsReduced(t *testing.T) {
	d := NewDesktop()
	file := NewMenu("&File")
	file.AddItem(NewMenuItem("New"))
	app := &mockApp{name: "Demo", menus: []*Menu{file}}
	d.activeApp = app

	// Full bar (no main window): system Psi menu + the app's first menu
	// carrying the merged Hide + Quit sections.
	d.updateMenuBarContent()
	if got := menuTitles(d.menuBar); len(got) != 2 || got[0] != "Ψ" || got[1] != "File" {
		t.Fatalf("full bar titles = %v, want [Ψ File]", got)
	}
	appMenu := d.menuBar.Menus()[1]
	if !menuHasItem(appMenu, "Hide Demo") || !menuHasItem(appMenu, "Quit Demo") {
		t.Errorf("full app menu missing Hide/Quit: %v", itemTexts(appMenu))
	}

	// Reduced bar (main window detached): Psi (Hide only) + Window
	// (Tile/Cascade). No Quit on the desktop; the app's file menu is
	// gone (it moves to the detached window's own bar in stage 2).
	main := window.NewWindow("main")
	main.SetDetached(true)
	app.main = main
	d.updateMenuBarContent()
	if got := menuTitles(d.menuBar); len(got) != 2 || got[0] != "Ψ" || got[1] != "Window" {
		t.Fatalf("reduced bar titles = %v, want [Ψ Window]", got)
	}
	psi := d.menuBar.Menus()[0]
	if !menuHasItem(psi, "Hide Demo") || !menuHasItem(psi, "Hide Others") || !menuHasItem(psi, "Show All") {
		t.Errorf("reduced Psi menu missing hide section: %v", itemTexts(psi))
	}
	if menuHasItem(psi, "Quit") {
		t.Errorf("reduced Psi menu should not carry Quit: %v", itemTexts(psi))
	}
	win := d.menuBar.Menus()[1]
	if !menuHasItem(win, "Tile") || !menuHasItem(win, "Cascade") {
		t.Errorf("reduced Window menu missing Tile/Cascade: %v", itemTexts(win))
	}
}

// The detached window's own first menu keeps its items and gains only
// the Quit section - no Hide section, no offset separator.
func TestDetachedWindowFirstMenuQuitOnly(t *testing.T) {
	d := NewDesktop()
	edit := NewMenu("&Edit")
	edit.AddItem(NewMenuItem("Copy"))
	m := d.createAppMenuWithQuitOnly(edit, "Demo")
	if !menuHasItem(m, "Copy") {
		t.Error("quit-only menu dropped the original items")
	}
	if !menuHasItem(m, "Quit Demo") {
		t.Error("quit-only menu missing Quit")
	}
	if menuHasItem(m, "Hide Demo") {
		t.Error("quit-only menu should not carry the Hide section")
	}
}

func itemTexts(m *Menu) []string {
	var out []string
	for _, it := range m.Items() {
		out = append(out, it.Text)
	}
	return out
}
