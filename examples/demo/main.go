// Package main demonstrates the TUI toolkit capabilities.
package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"sync"

	"github.com/phroun/tuitk/app"
	"github.com/phroun/tuitk/backend"
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/layout"
	"github.com/phroun/tuitk/protocol"
	"github.com/phroun/tuitk/style"
	"github.com/phroun/tuitk/widgets"
	"github.com/phroun/tuitk/window"
)

// fixedWidthBox is a bordered panel whose width is pinned, so its
// content width does not grow when the font changes. Word wrap only
// happens when width is genuinely constrained; an unconstrained label's
// SizeHint scales with the font and simply widens instead of wrapping.
// Height is NOT pinned: it flows from the content via height-for-width,
// so wrapping onto more lines makes the box taller.
type fixedWidthBox struct {
	*widgets.Panel
	width core.Unit
}

func newFixedWidthBox(width core.Unit, content core.Widget) *fixedWidthBox {
	f := &fixedWidthBox{Panel: widgets.NewPanel(), width: width}
	f.SetBorder(true)
	f.SetBorderStyle(style.BorderSingle) // zero-value BorderStyle renders invisibly

	boxLayout := layout.NewBoxLayout(core.Vertical)
	f.AddChild(content)
	f.SetLayoutManager(boxLayout)
	boxLayout.ItemAt(0).WithAlign(core.AlignFill)
	return f
}

func (f *fixedWidthBox) SizeHint() core.UnitSize {
	return core.UnitSize{Width: f.width, Height: f.Panel.SizeHint().Height}
}

// idCaptureFactory records built targets by object ID so the app can
// reach the real widgets behind surfaced reply names.
type idCaptureFactory struct {
	inner protocol.Factory
	byID  map[uint64]any
}

func (f *idCaptureFactory) New(typeName string) (protocol.Object, error) {
	o, err := f.inner.New(typeName)
	if err != nil {
		return nil, err
	}
	type built interface {
		Target() any
		ID() uint64
	}
	if b, ok := o.(built); ok {
		f.byID[b.ID()] = b.Target()
	}
	return o, nil
}

// Forward EventControl so the script's sub statements and D20 echo
// suppression reach the wrapped RegistryFactory.
func (f *idCaptureFactory) Subscribe(id uint64, typ string) {
	if ec, ok := f.inner.(protocol.EventControl); ok {
		ec.Subscribe(id, typ)
	}
}
func (f *idCaptureFactory) Unsubscribe(id uint64, typ string) {
	if ec, ok := f.inner.(protocol.EventControl); ok {
		ec.Unsubscribe(id, typ)
	}
}
func (f *idCaptureFactory) Suppressed(fn func()) {
	if ec, ok := f.inner.(protocol.EventControl); ok {
		ec.Suppressed(fn)
		return
	}
	fn()
}

// protocolWindowScript is the Protocol Demo window's entire content,
// expressed in the display-protocol command language (D10-D18).
const protocolWindowScript = `
alias C="caption"

root=new panel layout=vbox children={
	new label C="This window's content was built from protocol text." wrap
	status=new label C="Interact below; events appear here."
	new separator
	cb=new checkbox C="Tri-state checkbox (watch the label above)" tristate
	inp=new textinput placeholder="Type here..."
	combo=new combobox children={new item C="Alpha"; new item C="Beta"; new item C="Gamma"} selected=0
	btn=new button C="Dispatch demo.hello" action=demo.hello
}
watch=root.status
wcb=root.cb
winp=root.inp
wcombo=root.combo

# D20 default-closed: open the event flows this window listens to
# (command events need no sub - the button works regardless).
sub wcb toggle
sub winp change
sub wcombo change
`

// createProtocolWindow builds the P0 step-4 window: content
// constructed by executing protocol text, interactions delivered back
// as protocol event records into app handlers.
func createProtocolWindow(application *app.Application, desktop *widgets.Desktop) *window.Window {
	dispatcher := protocol.NewEventDispatcher()
	ctx := &protocol.BindContext{
		Dispatch: func(id string) { application.Commands().Dispatch(id) },
		Emit:     func(ev *protocol.Event) { dispatcher.Dispatch(ev) },
	}

	factory := &idCaptureFactory{
		inner: protocol.NewRegistryFactory(ctx),
		byID:  make(map[uint64]any),
	}

	script, err := protocol.Parse(protocolWindowScript)
	if err != nil {
		return nil
	}
	reply, err := protocol.NewSession().Execute(script, factory)
	if err != nil {
		return nil
	}

	rootWidget, _ := factory.byID[reply.IDs["root"]].(core.Widget)
	status, _ := factory.byID[reply.IDs["watch"]].(*widgets.Label)
	if rootWidget == nil || status == nil {
		return nil
	}

	// App-side handlers, keyed by the surfaced ObjectIDs - the same
	// records a remote display service would deliver.
	dispatcher.On(reply.IDs["wcb"], "toggle", func(ev *protocol.Event) {
		state := "off"
		switch ev.Flag("checked") {
		case protocol.FlagTrue:
			state = "on"
		case protocol.FlagIndeterminate:
			state = "mixed"
		}
		status.SetText("event toggle checked=" + state)
	})
	dispatcher.On(reply.IDs["winp"], "change", func(ev *protocol.Event) {
		if s, ok := ev.Text("text"); ok {
			status.SetText(`event change text="` + s + `"`)
		}
	})
	dispatcher.On(reply.IDs["wcombo"], "change", func(ev *protocol.Event) {
		if i, ok := ev.Int("selected"); ok {
			status.SetText(fmt.Sprintf("event change selected=%d", i))
		}
	})
	dispatcher.OnType("command", func(ev *protocol.Event) {
		if a, ok := ev.Word("action"); ok {
			status.SetText("event command action=" + a)
		}
	})

	// The command also lands in the app's registry (slice-1 seam).
	application.Commands().Register("demo.hello", func() {
		if sb := desktop.StatusBar(); sb != nil {
			sb.SetText("demo.hello dispatched from protocol-built button!")
		}
	})

	w := window.NewWindow("Protocol Demo")
	w.SetBounds(core.UnitRect{X: 8 * 8, Y: 16 * 4, Width: 8 * 56, Height: 16 * 16})
	w.SetContent(rootWidget)
	return w
}

func main() {
	// Create the TUI backend
	opts := backend.DefaultTUIOptions()
	tuiBackend := backend.NewTUIBackend(opts)

	// Create desktop - owns the backend and runs the event loop
	desktop := widgets.NewDesktop()
	desktop.SetBackend(tuiBackend)

	// Create the application - owns windows, provides menu/status content
	application := app.New(nil) // nil backend - Desktop owns it now
	application.SetName("TUI Demo")

	// Set up application's menu bar content
	application.SetMenuBarContent(createMenus(desktop, application))

	// Set up application's status bar content
	redStyle := style.DefaultStyle().WithFg(style.ColorRed).WithBg(style.ColorWhite)
	normalStatusContent := []widgets.StatusSection{
		{Spans: []widgets.StatusTextSpan{
			{Text: "Ready - Press "},
			{Text: "F10", Style: &redStyle},
			{Text: " for menu, Tab to navigate, "},
			{Text: "Ctrl+Q", Style: &redStyle},
			{Text: " to quit"},
		}},
	}
	application.SetStatusBarContent(normalStatusContent)

	// Register application with desktop
	desktop.AddApplication(application)

	// Add event filter for debugging (shows key presses in status bar)
	desktop.AddEventFilter(func(event core.Event) bool {
		if keyEvent, ok := event.(core.KeyPressEvent); ok {
			wm := desktop.WindowManager()
			focusInfo := "no window"
			winInfo := ""
			if wm != nil {
				if activeWin := wm.ActiveWindow(); activeWin != nil {
					fm := activeWin.FocusManager()
					if fm != nil {
						chain := fm.FocusChain()
						focused := fm.FocusedWidget()

						focusable := 0
						for _, w := range chain {
							if w.IsVisible() && w.IsEnabled() {
								focusable++
							}
						}

						if focused != nil {
							focusInfo = fmt.Sprintf("%T chain:%d ok:%d", focused, len(chain), focusable)
						} else {
							focusInfo = fmt.Sprintf("nil chain:%d ok:%d", len(chain), focusable)
						}
					}

					if keyEvent.Key == "Tab" || keyEvent.Key == "Shift+Tab" {
						bounds := activeWin.Bounds()
						offset := activeWin.ClientAreaOffset()
						state := "normal"
						if activeWin.IsMaximized() {
							state = "MAX"
						}
						winInfo = fmt.Sprintf(" | win:%dx%d@%d,%d content:@%d,%d %s",
							bounds.Width, bounds.Height, bounds.X, bounds.Y,
							offset.X, offset.Y, state)
					}
				}
			}
			if statusBar := desktop.StatusBar(); statusBar != nil {
				statusBar.SetText(fmt.Sprintf("Key: %q  %s%s", keyEvent.Key, focusInfo, winInfo))
			}
		}
		return false // Don't consume the event
	})

	// Create windows in startup callback (after screen bounds are set)
	desktop.SetOnStartup(func() {
		// P0 step 4: a window whose content is built entirely from
		// protocol text, with interactions flowing back as event
		// records. See createProtocolWindow.
		if pw := createProtocolWindow(application, desktop); pw != nil {
			application.AddWindow(pw)
		}
		// Create the main demo window - owned by the application
		mainWindow := createMainWindow(desktop, application)
		application.AddWindow(mainWindow)
	})

	// Run the desktop event loop
	desktop.Run()
}

// createMenus creates the application's menu bar content.
// The first menu is named after the app - standard items (Hide, Show All, Quit)
// are automatically appended by the Desktop.
func createMenus(desktop *widgets.Desktop, application *app.Application) []*widgets.Menu {
	var menus []*widgets.Menu

	// App menu (named after the application - standard items added automatically)
	appMenu := widgets.NewMenu("&Demo")
	newItem := widgets.NewMenuItem("&New")
	newItem.SetShortcut(core.NewShortcut("^N"))
	appMenu.AddItem(newItem)

	openItem := widgets.NewMenuItem("&Open...")
	openItem.SetShortcut(core.NewShortcut("^O"))
	appMenu.AddItem(openItem)

	saveItem := widgets.NewMenuItem("&Save")
	saveItem.SetShortcut(core.NewShortcut("^S"))
	appMenu.AddItem(saveItem)

	// Note: Hide, Hide Others, Show All, and Quit are added automatically
	menus = append(menus, appMenu)

	// Edit menu
	editMenu := widgets.NewMenu("&Edit")
	cutItem := widgets.NewMenuItem("Cu&t")
	cutItem.SetShortcut(core.NewShortcut("^X"))
	editMenu.AddItem(cutItem)

	copyItem := widgets.NewMenuItem("&Copy")
	copyItem.SetShortcut(core.NewShortcut("^C"))
	editMenu.AddItem(copyItem)

	pasteItem := widgets.NewMenuItem("&Paste")
	pasteItem.SetShortcut(core.NewShortcut("^V"))
	editMenu.AddItem(pasteItem)

	editMenu.AddSeparator()

	// Raw Key Input - passes the next keypress directly to the focused widget
	rawKeyItem := widgets.NewMenuItem("&Raw Key Input")
	rawKeyItem.SetShortcut(core.NewShortcut("^\\"))
	rawKeyItem.SetOnTriggered(func() {
		desktop.ActivatePassNextKeyToWidget()
	})
	editMenu.AddItem(rawKeyItem)

	menus = append(menus, editMenu)

	// View menu
	viewMenu := widgets.NewMenu("&View")
	toolbarItem := widgets.NewMenuItem("&Toolbar")
	toolbarItem.SetCheckable(true)
	toolbarItem.SetChecked(true)
	viewMenu.AddItem(toolbarItem)

	statusBarItem := widgets.NewMenuItem("&Status Bar")
	statusBarItem.SetCheckable(true)
	statusBarItem.SetChecked(true)
	viewMenu.AddItem(statusBarItem)

	viewMenu.AddSeparator()

	// Track accessibility settings
	var showVisualAnnouncements, speakAnnouncements bool

	// Track current speech process so we can interrupt it
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
			// Both disabled
			am.OnAnnounce = nil
			return
		}

		am.OnAnnounce = func(announcement core.AccessibilityAnnouncement) {
			// Visual output to status bar
			if showVisualAnnouncements {
				if statusBar := desktop.StatusBar(); statusBar != nil {
					prefix := "📢"
					if announcement.Priority == "assertive" {
						prefix = "⚠️"
					}
					statusBar.SetText(fmt.Sprintf("%s [%s] %s", prefix, announcement.Priority, announcement.Message))
				}
			}

			// Text-to-speech on macOS using 'say' command
			if speakAnnouncements && runtime.GOOS == "darwin" {
				go func(msg string) {
					speechMu.Lock()
					// Kill any previous speech
					if speechCmd != nil && speechCmd.Process != nil {
						_ = speechCmd.Process.Kill()
						_ = speechCmd.Wait()
					}
					// Start new speech
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

	// Screen reader visual output toggle
	screenReaderItem := widgets.NewMenuItem("Show A&nnouncements in Status Bar")
	screenReaderItem.SetCheckable(true)
	screenReaderItem.SetChecked(false)
	screenReaderItem.SetOnTriggered(func() {
		showVisualAnnouncements = screenReaderItem.Checked
		updateAccessibilityHandler()
		if showVisualAnnouncements {
			if am := desktop.AccessibilityManager(); am != nil {
				am.AnnouncePolite("Visual announcements enabled")
			}
		}
	})
	viewMenu.AddItem(screenReaderItem)

	// Text-to-speech toggle (macOS only)
	speakItem := widgets.NewMenuItem("Speak Announcements (macOS)")
	speakItem.SetCheckable(true)
	speakItem.SetChecked(false)
	if runtime.GOOS != "darwin" {
		speakItem.SetEnabled(false)
	}
	speakItem.SetOnTriggered(func() {
		speakAnnouncements = speakItem.Checked
		updateAccessibilityHandler()
		if speakAnnouncements {
			if am := desktop.AccessibilityManager(); am != nil {
				am.AnnouncePolite("Text to speech enabled")
			}
		}
	})
	viewMenu.AddItem(speakItem)

	menus = append(menus, viewMenu)

	// Window menu
	windowMenu := widgets.NewMenu("&Window")
	newWindowItem := widgets.NewMenuItem("&New Window")
	newWindowItem.SetOnTriggered(func() {
		// Create a NEW application with its own identity, menus, and status bar
		newApp := createSecondaryApplication(desktop)
		desktop.AddApplication(newApp)
	})
	windowMenu.AddItem(newWindowItem)

	windowMenu.AddSeparator()

	tileItem := widgets.NewMenuItem("&Tile")
	tileItem.SetOnTriggered(func() {
		desktop.WindowManager().TileWindows()
	})
	windowMenu.AddItem(tileItem)

	cascadeItem := widgets.NewMenuItem("&Cascade")
	cascadeItem.SetOnTriggered(func() {
		desktop.WindowManager().CascadeWindows()
	})
	windowMenu.AddItem(cascadeItem)
	menus = append(menus, windowMenu)

	// Alphabet menu (26 items for testing scrolling)
	alphabetMenu := widgets.NewMenu("&Alphabet")
	for i := 0; i < 26; i++ {
		letter := string(rune('A' + i))
		item := widgets.NewMenuItem("&" + letter + " - Letter " + letter)
		alphabetMenu.AddItem(item)
	}
	menus = append(menus, alphabetMenu)

	// Help menu
	helpMenu := widgets.NewMenu("&Help")
	aboutItem := widgets.NewMenuItem("&About")
	aboutItem.SetOnTriggered(func() {
		showAboutDialog(desktop, application)
	})
	helpMenu.AddItem(aboutItem)
	menus = append(menus, helpMenu)

	return menus
}

// createDemoWindow creates a simple demo window with an embedded terminal.
func createDemoWindow(desktop *widgets.Desktop, application *app.Application) *window.Window {
	w := window.NewWindow("Demo Window")
	w.SetSize(core.UnitSize{
		Width:  core.Unit(60 * 8),
		Height: core.Unit(20 * 16),
	})

	// Create a vertical splitter to divide the window
	splitter := widgets.NewVSplitter()
	splitter.SetTitle("Terminal")
	splitter.SetPosition(0.3) // Top panel gets 30% of space

	// Top panel with controls
	topPanel := widgets.NewPanel()
	boxLayout := layout.NewBoxLayout(core.Vertical)
	boxLayout.SetSpacing(8)

	label := widgets.NewLabel("This is a child window.")
	topPanel.AddChild(label)

	textInput := widgets.NewTextInput()
	textInput.SetPlaceholder("Type something...")
	topPanel.AddChild(textInput)

	closeButton := widgets.NewButton("Close")
	closeButton.SetOnClick(func() {
		w.Close()
	})
	topPanel.AddChild(closeButton)

	topPanel.SetLayoutManager(boxLayout)
	splitter.SetFirst(topPanel)

	// Bottom panel with PurfecTerm terminal
	terminal := widgets.NewPurfecTerm()
	splitter.SetSecond(terminal)

	// Start the terminal shell
	terminal.Start()

	w.SetContent(splitter)

	return w
}

// MDI child window counter for unique naming
var mdiChildCount int

// createMDIDemo creates an MDIPane widget demonstration.
// The MDIPane is a reusable widget that can be embedded anywhere.
func createMDIDemo(desktop *widgets.Desktop, application *app.Application, parentWindow *window.Window) core.Widget {
	// Use a VSplitter to divide space between MDIPane and DockRow
	splitter := widgets.NewVSplitter()
	splitter.SetPosition(0.9) // MDIPane gets 90%, DockRow gets 10%
	splitter.SetTitle("Dock")

	// Create an MDIPane - this is a widget that can be embedded in tabs, splitters, etc.
	mdiPane := widgets.NewMDIPane()
	mdiPane.SetBackgroundChar('░') // Light shade pattern

	// Set fixed size using Units (80 columns x 25 rows)
	// To fix both dimensions, set min and max to the same value
	metrics := core.DefaultCellMetrics()
	fixedSize := core.UnitSize{
		Width:  80 * metrics.CellWidth,
		Height: 25 * metrics.CellHeight,
	}
	mdiPane.SetMinimumSize(fixedSize)
	mdiPane.SetMaximumSize(fixedSize)

	// Wrap the MDIPane in a ScrollArea for scrolling when larger than view
	mdiScrollArea := widgets.NewScrollArea()
	mdiScrollArea.SetContent(mdiPane)

	// Create a dock row at the bottom for minimized windows
	dockRow := widgets.NewDockRow()
	dockRow.SetEntryWidth(20)

	// Track minimized windows to their dock entries
	dockEntries := make(map[*window.Window]*widgets.DockEntry)

	// Wire up minimize callback to add dock entries
	mdiPane.SetOnWindowMinimized(func(win *window.Window) {
		entry := &widgets.DockEntry{
			Title: win.Title(),
			OnClick: func() {
				mdiPane.RestoreWindow(win)
			},
		}
		dockEntries[win] = entry
		dockRow.AddEntry(entry)
	})

	// Wire up restore callback to remove dock entries
	mdiPane.SetOnWindowRestored(func(win *window.Window) {
		if entry, ok := dockEntries[win]; ok {
			dockRow.RemoveEntry(entry)
			delete(dockEntries, win)
		}
	})

	// Also remove from dock when window is closed
	mdiPane.SetOnWindowRemoved(func(win *window.Window) {
		if entry, ok := dockEntries[win]; ok {
			dockRow.RemoveEntry(entry)
			delete(dockEntries, win)
		}
	})

	// Create a control panel as background content
	controlPanel := widgets.NewPanel()
	controlLayout := layout.NewBoxLayout(core.Vertical)
	controlLayout.SetSpacing(8)

	// Description
	descLabel := widgets.NewLabel("MDIPane Widget Demo")
	controlPanel.AddChild(descLabel)

	infoLabel := widgets.NewLabel("This MDIPane widget manages floating windows.\nClick [_] to minimize windows to the dock below.")
	controlPanel.AddChild(infoLabel)

	// Button to spawn new MDI child window
	spawnButton := widgets.NewButton("Spawn Window in MDIPane")
	spawnButton.SetOnClick(func() {
		mdiChildCount++
		childWindow := createMDIPaneChildWindow(mdiPane, mdiChildCount)
		mdiPane.AddWindow(childWindow)
	})
	controlPanel.AddChild(spawnButton)

	// Button row for window management
	buttonPanel := widgets.NewPanel()
	hLayout := layout.NewBoxLayout(core.Horizontal)
	hLayout.SetSpacing(8)

	tileButton := widgets.NewButton("Tile")
	tileButton.SetOnClick(func() {
		mdiPane.TileWindows()
	})
	buttonPanel.AddChild(tileButton)

	cascadeButton := widgets.NewButton("Cascade")
	cascadeButton.SetOnClick(func() {
		mdiPane.CascadeWindows()
	})
	buttonPanel.AddChild(cascadeButton)

	nextButton := widgets.NewButton("Next")
	nextButton.SetOnClick(func() {
		mdiPane.NextWindow()
	})
	buttonPanel.AddChild(nextButton)

	prevButton := widgets.NewButton("Prev")
	prevButton.SetOnClick(func() {
		mdiPane.PrevWindow()
	})
	buttonPanel.AddChild(prevButton)

	buttonPanel.SetLayoutManager(hLayout)
	controlPanel.AddChild(buttonPanel)

	// Status label showing active window
	statusLabel := widgets.NewLabel("Active: none")
	controlPanel.AddChild(statusLabel)

	// Update status when active window changes
	mdiPane.SetOnActiveWindowChanged(func(win *window.Window) {
		if win != nil {
			statusLabel.SetText(fmt.Sprintf("Active: %s", win.Title()))
		} else {
			statusLabel.SetText("Active: none")
		}
	})

	// Add a spacer
	spacer := widgets.NewSpacer()
	controlPanel.AddChild(spacer)

	// Tips
	tipLabel := widgets.NewLabel("Tips:")
	controlPanel.AddChild(tipLabel)

	tip1 := widgets.NewLabel("- Click [_] to minimize to dock")
	controlPanel.AddChild(tip1)

	tip2 := widgets.NewLabel("- Click dock entry to restore")
	controlPanel.AddChild(tip2)

	tip3 := widgets.NewLabel("- Double-click title to maximize")
	controlPanel.AddChild(tip3)

	controlPanel.SetLayoutManager(controlLayout)

	// Set the control panel as background content of the MDI pane
	mdiPane.SetContent(controlPanel)

	// Set up the splitter: ScrollArea (containing MDIPane) on top, DockRow on bottom
	splitter.SetFirst(mdiScrollArea)
	splitter.SetSecond(dockRow)

	// Spawn an initial window to show capabilities
	mdiChildCount++
	initialWindow := createMDIPaneChildWindow(mdiPane, mdiChildCount)
	mdiPane.AddWindow(initialWindow)

	return splitter
}

// createMDIPaneChildWindow creates a window for use in an MDIPane.
func createMDIPaneChildWindow(mdiPane *widgets.MDIPane, id int) *window.Window {
	w := window.NewWindow(fmt.Sprintf("Document %d", id))

	// Position with cascade offset
	offset := (id - 1) % 5
	w.SetBounds(core.UnitRect{
		X:      core.Unit((offset*2 + 1) * 8),
		Y:      core.Unit((offset + 1) * 16),
		Width:  core.Unit(30 * 8),
		Height: core.Unit(8 * 16),
	})

	panel := widgets.NewPanel()
	boxLayout := layout.NewBoxLayout(core.Vertical)
	boxLayout.SetSpacing(8)

	label := widgets.NewLabel(fmt.Sprintf("Document #%d", id))
	panel.AddChild(label)

	textInput := widgets.NewTextInput()
	textInput.SetPlaceholder("Enter document content...")
	panel.AddChild(textInput)

	buttonPanel := widgets.NewPanel()
	hLayout := layout.NewBoxLayout(core.Horizontal)
	hLayout.SetSpacing(8)

	newButton := widgets.NewButton("New")
	newButton.SetOnClick(func() {
		mdiChildCount++
		newWin := createMDIPaneChildWindow(mdiPane, mdiChildCount)
		mdiPane.AddWindow(newWin)
	})
	buttonPanel.AddChild(newButton)

	closeButton := widgets.NewButton("Close")
	closeButton.SetOnClick(func() {
		mdiPane.RemoveWindow(w)
	})
	buttonPanel.AddChild(closeButton)

	buttonPanel.SetLayoutManager(hLayout)
	panel.AddChild(buttonPanel)

	panel.SetLayoutManager(boxLayout)
	w.SetContent(panel)

	return w
}

// createMDIChildWindow creates a simple MDI child window.
func createMDIChildWindow(parentWindow *window.Window, id int) *window.Window {
	w := window.NewWindow(fmt.Sprintf("MDI Child %d", id))

	// Offset each child window slightly for cascading effect
	offset := (id - 1) % 5
	w.SetBounds(core.UnitRect{
		X:      core.Unit((offset*3 + 2) * 8),
		Y:      core.Unit((offset*2 + 2) * 16),
		Width:  core.Unit(35 * 8),  // 35 columns
		Height: core.Unit(10 * 16), // 10 rows
	})

	panel := widgets.NewPanel()
	boxLayout := layout.NewBoxLayout(core.Vertical)
	boxLayout.SetSpacing(8)

	label := widgets.NewLabel(fmt.Sprintf("This is MDI Child Window #%d", id))
	panel.AddChild(label)

	textInput := widgets.NewTextInput()
	textInput.SetPlaceholder("Enter some text...")
	panel.AddChild(textInput)

	// Button row
	buttonPanel := widgets.NewPanel()
	hLayout := layout.NewBoxLayout(core.Horizontal)
	hLayout.SetSpacing(8)

	closeButton := widgets.NewButton("Close")
	closeButton.SetOnClick(func() {
		w.Close()
	})
	buttonPanel.AddChild(closeButton)

	spawnButton := widgets.NewButton("Spawn Another")
	spawnButton.SetOnClick(func() {
		mdiChildCount++
		childWindow := createMDIChildWindow(parentWindow, mdiChildCount)
		childWindow.SetParentWindow(parentWindow)
	})
	buttonPanel.AddChild(spawnButton)

	buttonPanel.SetLayoutManager(hLayout)
	panel.AddChild(buttonPanel)

	panel.SetLayoutManager(boxLayout)
	w.SetContent(panel)

	return w
}

// Secondary application counter for unique naming
var secondaryAppCount int

// createSecondaryApplication creates a new application with its own menus and status bar.
func createSecondaryApplication(desktop *widgets.Desktop) *app.Application {
	secondaryAppCount++
	appNum := secondaryAppCount

	// Create new application (nil backend - Desktop owns it)
	newApp := app.New(nil)
	newApp.SetName(fmt.Sprintf("Secondary App %d", appNum))

	// Create simple menu bar for this application
	menus := createSecondaryMenus(desktop, newApp, appNum)
	newApp.SetMenuBarContent(menus)

	// Create unique status bar content
	newApp.SetStatusBarContent([]widgets.StatusSection{
		{Spans: []widgets.StatusTextSpan{
			{Text: fmt.Sprintf("Secondary Application #%d", appNum)},
		}},
	})

	// Create window for this application
	w := window.NewWindow(fmt.Sprintf("App %d Window", appNum))
	offset := (appNum - 1) % 5
	w.SetBounds(core.UnitRect{
		X:      core.Unit((offset*3 + 5) * 8),
		Y:      core.Unit((offset*2 + 3) * 16),
		Width:  core.Unit(60 * 8),
		Height: core.Unit(20 * 16),
	})

	// Create a vertical splitter to divide the window
	splitter := widgets.NewVSplitter()
	splitter.SetTitle("Terminal")
	splitter.SetPosition(0.3) // Top panel gets 30% of space

	// Top panel with controls
	topPanel := widgets.NewPanel()
	boxLayout := layout.NewBoxLayout(core.Vertical)
	boxLayout.SetSpacing(8)

	label := widgets.NewLabel(fmt.Sprintf("This window belongs to Application #%d", appNum))
	topPanel.AddChild(label)

	infoLabel := widgets.NewLabel("Notice the menu bar and status bar change\nwhen this window is focused.")
	topPanel.AddChild(infoLabel)

	textInput := widgets.NewTextInput()
	textInput.SetPlaceholder("Enter text here...")
	topPanel.AddChild(textInput)

	closeButton := widgets.NewButton("Close Window")
	closeButton.SetOnClick(func() {
		w.Close()
	})
	topPanel.AddChild(closeButton)

	topPanel.SetLayoutManager(boxLayout)
	splitter.SetFirst(topPanel)

	// Bottom panel with PurfecTerm terminal
	terminal := widgets.NewPurfecTerm()

	// Debug callback - show clicked cell info in status bar
	terminal.SetOnCellClicked(func(info widgets.CellDebugInfo) {
		// Format attributes (B=bold, U=underline, R=reverse)
		attrs := ""
		if info.Bold {
			attrs += "B"
		}
		if info.Underline {
			attrs += "U"
		}
		if info.Reverse {
			attrs += "R"
		}
		if attrs == "" {
			attrs = "-"
		}

		// Format colors
		var fg, bg string
		switch info.FgType {
		case "RGB":
			fg = fmt.Sprintf("RGB(%d,%d,%d)", info.FgR, info.FgG, info.FgB)
		case "256":
			fg = fmt.Sprintf("256[%d]", info.FgIndex)
		case "Std":
			fg = fmt.Sprintf("Std[%d]", info.FgIndex)
		default:
			fg = "Def"
		}
		switch info.BgType {
		case "RGB":
			bg = fmt.Sprintf("RGB(%d,%d,%d)", info.BgR, info.BgG, info.BgB)
		case "256":
			bg = fmt.Sprintf("256[%d]", info.BgIndex)
		case "Std":
			bg = fmt.Sprintf("Std[%d]", info.BgIndex)
		default:
			bg = "Def"
		}

		// Format character (handle non-printable)
		charStr := fmt.Sprintf("'%c'", info.Char)
		if info.Char < 32 || info.Char == 127 {
			charStr = fmt.Sprintf("0x%02X", info.Char)
		}

		newApp.SetStatusBarContent([]widgets.StatusSection{
			{Text: fmt.Sprintf("[%d,%d] %s Fg:%s Bg:%s Attr:%s",
				info.Col, info.Row, charStr, fg, bg, attrs)},
		})
	})

	splitter.SetSecond(terminal)

	// Start the terminal shell
	terminal.Start()

	w.SetContent(splitter)

	newApp.AddWindow(w)

	return newApp
}

// createSecondaryMenus creates a simple menu bar for secondary applications.
// The first menu is named after the app - standard items are added automatically.
func createSecondaryMenus(desktop *widgets.Desktop, application *app.Application, appNum int) []*widgets.Menu {
	var menus []*widgets.Menu

	// App menu (named after the application - standard items added automatically)
	appMenu := widgets.NewMenu(fmt.Sprintf("&App %d", appNum))
	closeItem := widgets.NewMenuItem("&Close Window")
	closeItem.SetShortcut(core.NewShortcut("^W"))
	closeItem.SetOnTriggered(func() {
		// Close the first window of this application
		windows := application.Windows()
		if len(windows) > 0 {
			windows[0].Close()
		}
	})
	appMenu.AddItem(closeItem)

	// Note: Hide, Hide Others, Show All, and Quit are added automatically
	menus = append(menus, appMenu)

	// Edit menu (for testing Raw Key Input in secondary apps)
	editMenu := widgets.NewMenu("&Edit")
	cutItem := widgets.NewMenuItem("Cu&t")
	cutItem.SetShortcut(core.NewShortcut("^X"))
	editMenu.AddItem(cutItem)

	copyItem := widgets.NewMenuItem("&Copy")
	copyItem.SetShortcut(core.NewShortcut("^C"))
	editMenu.AddItem(copyItem)

	pasteItem := widgets.NewMenuItem("&Paste")
	pasteItem.SetShortcut(core.NewShortcut("^V"))
	editMenu.AddItem(pasteItem)

	editMenu.AddSeparator()

	// Raw Key Input - passes the next keypress directly to the focused widget
	rawKeyItem := widgets.NewMenuItem("&Raw Key Input")
	rawKeyItem.SetShortcut(core.NewShortcut("^\\"))
	rawKeyItem.SetOnTriggered(func() {
		desktop.ActivatePassNextKeyToWidget()
	})
	editMenu.AddItem(rawKeyItem)
	menus = append(menus, editMenu)

	// Info menu (app-specific)
	infoMenu := widgets.NewMenu("&Info")
	infoItem := widgets.NewMenuItem("&About This App")
	infoItem.SetOnTriggered(func() {
		dialog := widgets.NewMessageBox(
			fmt.Sprintf("About App %d", appNum),
			fmt.Sprintf("This is Secondary Application #%d\n\nIt has its own menus and status bar.", appNum),
			widgets.ButtonOK,
		)
		dialog.SetIcon(widgets.IconInformation)
		application.AddWindow(&dialog.Window)
	})
	infoMenu.AddItem(infoItem)
	menus = append(menus, infoMenu)

	// Help menu
	helpMenu := widgets.NewMenu("&Help")
	aboutItem := widgets.NewMenuItem("&About")
	aboutItem.SetOnTriggered(func() {
		dialog := widgets.NewMessageBox(
			"About",
			"Secondary Application\n\nDemonstrates multi-application support.",
			widgets.ButtonOK,
		)
		dialog.SetIcon(widgets.IconInformation)
		application.AddWindow(&dialog.Window)
	})
	helpMenu.AddItem(aboutItem)
	menus = append(menus, helpMenu)

	return menus
}

// showAboutDialog shows the about dialog.
func showAboutDialog(desktop *widgets.Desktop, application *app.Application) {
	dialog := widgets.NewMessageBox(
		"About TUI Toolkit",
		"TUI Toolkit Demo\n\nA comprehensive terminal UI framework.\n\nVersion 0.1.0",
		widgets.ButtonOK,
	)
	dialog.SetIcon(widgets.IconInformation)
	dialog.SetOnFinished(func(result widgets.DialogResult) {
		// Dialog closed
	})

	// MessageBox is itself a window, add it to the application
	application.AddWindow(&dialog.Window)
}
