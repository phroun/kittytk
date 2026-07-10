package trinkets

import (
	"strings"
	"testing"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/window"
)

// mockApp is a minimal ApplicationProvider for exercising the desktop's
// menu-bar composition.
type mockApp struct {
	name     string
	menuName string
	main     *window.Window
	menus    []*Menu
	windows  []*window.Window
}

func (a *mockApp) Name() string { return a.name }
func (a *mockApp) MenuName() string {
	if a.menuName == "" {
		return "≡"
	}
	return a.menuName
}
func (a *mockApp) Windows() []*window.Window         { return a.windows }
func (a *mockApp) MainWindow() *window.Window        { return a.main }
func (a *mockApp) AddWindow(*window.Window)          {}
func (a *mockApp) RemoveWindow(*window.Window)       {}
func (a *mockApp) MenuBarContent() []*Menu           { return a.menus }
func (a *mockApp) StatusBarContent() []StatusSection { return nil }
func (a *mockApp) OnActivate()                       {}
func (a *mockApp) OnDeactivate()                     {}
func (a *mockApp) SetDesktop(core.Trinket)           {}
func (a *mockApp) PassNextKeyToTrinket() bool        { return false }
func (a *mockApp) ActivatePassNextKeyToTrinket()     {}
func (a *mockApp) ClearPassNextKeyToTrinket()        {}

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

	// Reduced bar (main window detached): the real system Psi menu, an
	// app-named menu carrying only the Hide section, and a Window menu
	// (Tile/Cascade). The real Psi keeps its own items (Exit Desktop);
	// the app's file menu moves to the detached window's own bar.
	main := window.NewWindow("main")
	main.SetDetached(true)
	app.main = main
	d.updateMenuBarContent()
	if got := menuTitles(d.menuBar); len(got) != 3 || got[0] != "Ψ" || got[1] != "Demo" || got[2] != "Window" {
		t.Fatalf("reduced bar titles = %v, want [Ψ Demo Window]", got)
	}
	psi := d.menuBar.Menus()[0]
	if psi != d.systemMenu {
		t.Errorf("reduced bar first menu should be the real system menu")
	}
	if !menuHasItem(psi, "Exit Desktop") {
		t.Errorf("reduced system menu lost its own items: %v", itemTexts(psi))
	}
	appHide := d.menuBar.Menus()[1]
	if !menuHasItem(appHide, "Hide Demo") || !menuHasItem(appHide, "Hide Others") || !menuHasItem(appHide, "Show All") {
		t.Errorf("reduced app menu missing hide section: %v", itemTexts(appHide))
	}
	if menuHasItem(appHide, "Quit") {
		t.Errorf("reduced app menu should not carry Quit: %v", itemTexts(appHide))
	}
	win := d.menuBar.Menus()[2]
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
	m := d.createAppMenuWithQuitOnly(edit, "≡", "Demo")
	if m.Title() != "≡" {
		t.Errorf("quit-only menu title = %q, want ≡", m.Title())
	}
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

// The desktop-hosted Window menu keeps its own custom entries, then lists
// the app's own windows, a separator, and the other in-surface desktop
// windows (belonging to other apps). Torn windows of other apps live on
// their own surfaces (not in the manager) and so never appear.
func TestDesktopWindowMenuListsAppThenOthers(t *testing.T) {
	d := NewDesktop()
	d.windowManager = window.NewWindowManager()

	winMenu := NewMenu("&Window")
	winMenu.AddItem(NewMenuItem("Zoom"))

	appWin1 := window.NewWindow("Doc 1")
	appWin2 := window.NewWindow("Doc 2")
	app := &mockApp{name: "Demo", windows: []*window.Window{appWin1, appWin2}}

	// An in-surface window belonging to some other app.
	other := window.NewWindow("Other Win")
	d.windowManager.AddWindow(other)

	menu := d.buildDesktopWindowMenu(winMenu, app)
	texts := itemTexts(menu)

	if len(texts) == 0 || texts[0] != "Zoom" {
		t.Fatalf("Window menu should lead with its custom entry, got %v", texts)
	}
	for _, want := range []string{"Doc 1", "Doc 2", "Other Win"} {
		if !menuHasItem(menu, want) {
			t.Errorf("Window menu missing %q: %v", want, texts)
		}
	}

	idx := func(s string) int {
		for i, it := range menu.Items() {
			if it.Text == s {
				return i
			}
		}
		return -1
	}
	if idx("Doc 1") > idx("Other Win") || idx("Doc 2") > idx("Other Win") {
		t.Errorf("app windows should precede other-app windows: %v", texts)
	}

	// The app's own windows are not duplicated when they are also in-surface.
	d.windowManager.AddWindow(appWin1)
	menu = d.buildDesktopWindowMenu(winMenu, app)
	count := 0
	for _, it := range menu.Items() {
		if it.Text == "Doc 1" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Doc 1 listed %d times, want 1: %v", count, itemTexts(menu))
	}
}

func itemTexts(m *Menu) []string {
	var out []string
	for _, it := range m.Items() {
		out = append(out, it.Text)
	}
	return out
}
