// Package main demonstrates the TUI toolkit capabilities.
package main

import (
	"fmt"
	"os"

	"github.com/phroun/tuitk/app"
	"github.com/phroun/tuitk/backend"
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/layout"
	"github.com/phroun/tuitk/widgets"
	"github.com/phroun/tuitk/window"
)

func main() {
	// Create the TUI backend with default options
	tuiBackend := backend.NewTUIBackend(backend.TUIOptions{
		Output: os.Stdout,
		Input:  os.Stdin,
	})

	// Create the application
	application := app.New(tuiBackend)

	// Create desktop with Mac-style menu bar at top
	desktop := widgets.NewDesktop()

	// Create the application menu bar (at desktop level, not per-window)
	menuBar := createMenuBar(application)
	desktop.SetMenuBar(menuBar)

	// Create a status bar at the bottom
	statusBar := widgets.NewStatusBar()
	statusBar.SetText("Ready - Press F10 for menu, Tab to navigate, Ctrl+Q to quit")
	desktop.SetStatusBar(statusBar)

	// Set desktop as the application's desktop widget
	application.SetDesktop(desktop)

	// Add event filter to show key presses in status bar (for debugging)
	application.AddEventFilter(func(event core.Event) bool {
		if keyEvent, ok := event.(core.KeyPressEvent); ok {
			// Show key and current focus info
			wm := application.WindowManager()
			focusInfo := "no window"
			if wm != nil {
				if activeWin := wm.ActiveWindow(); activeWin != nil {
					fm := activeWin.FocusManager()
					if fm != nil {
						chain := fm.FocusChain()
						focused := fm.FocusedWidget()

						// Count focusable widgets (visible + enabled)
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
				}
			}
			statusBar.SetText(fmt.Sprintf("Key: %q  %s", keyEvent.Key, focusInfo))
		}
		return false // Don't consume the event
	})

	// Create windows in startup callback (after screen bounds are set)
	application.SetOnStartup(func() {
		// Create the main demo window (floats over the desktop)
		mainWindow := createMainWindow(application, statusBar)
		application.WindowManager().AddWindow(mainWindow)
	})

	// Run the application
	application.Run()
}

// createMenuBar creates the application menu bar (Mac-style, at desktop level).
func createMenuBar(application *app.Application) *widgets.MenuBar {
	menuBar := widgets.NewMenuBar()

	// File menu
	fileMenu := widgets.NewMenu("File")
	newItem := widgets.NewMenuItem("New")
	newItem.SetShortcut(core.NewShortcut("^N"))
	fileMenu.AddItem(newItem)

	openItem := widgets.NewMenuItem("Open...")
	openItem.SetShortcut(core.NewShortcut("^O"))
	fileMenu.AddItem(openItem)

	saveItem := widgets.NewMenuItem("Save")
	saveItem.SetShortcut(core.NewShortcut("^S"))
	fileMenu.AddItem(saveItem)

	fileMenu.AddSeparator()

	exitItem := widgets.NewMenuItem("Exit")
	exitItem.SetShortcut(core.NewShortcut("^Q"))
	exitItem.SetOnTriggered(func() {
		application.Quit()
	})
	fileMenu.AddItem(exitItem)

	menuBar.AddMenu(fileMenu)

	// Edit menu
	editMenu := widgets.NewMenu("Edit")
	cutItem := widgets.NewMenuItem("Cut")
	cutItem.SetShortcut(core.NewShortcut("^X"))
	editMenu.AddItem(cutItem)

	copyItem := widgets.NewMenuItem("Copy")
	copyItem.SetShortcut(core.NewShortcut("^C"))
	editMenu.AddItem(copyItem)

	pasteItem := widgets.NewMenuItem("Paste")
	pasteItem.SetShortcut(core.NewShortcut("^V"))
	editMenu.AddItem(pasteItem)

	menuBar.AddMenu(editMenu)

	// View menu
	viewMenu := widgets.NewMenu("View")

	toolbarItem := widgets.NewMenuItem("Toolbar")
	toolbarItem.SetCheckable(true)
	toolbarItem.SetChecked(true)
	viewMenu.AddItem(toolbarItem)

	statusBarItem := widgets.NewMenuItem("Status Bar")
	statusBarItem.SetCheckable(true)
	statusBarItem.SetChecked(true)
	viewMenu.AddItem(statusBarItem)

	menuBar.AddMenu(viewMenu)

	// Window menu
	windowMenu := widgets.NewMenu("Window")

	newWindowItem := widgets.NewMenuItem("New Window")
	newWindowItem.SetOnTriggered(func() {
		demoWindow := createDemoWindow(application)
		application.WindowManager().AddWindow(demoWindow)
	})
	windowMenu.AddItem(newWindowItem)

	windowMenu.AddSeparator()

	tileItem := widgets.NewMenuItem("Tile")
	tileItem.SetOnTriggered(func() {
		application.WindowManager().TileWindows()
	})
	windowMenu.AddItem(tileItem)

	cascadeItem := widgets.NewMenuItem("Cascade")
	cascadeItem.SetOnTriggered(func() {
		application.WindowManager().CascadeWindows()
	})
	windowMenu.AddItem(cascadeItem)

	menuBar.AddMenu(windowMenu)

	// Help menu
	helpMenu := widgets.NewMenu("Help")

	aboutItem := widgets.NewMenuItem("About")
	aboutItem.SetOnTriggered(func() {
		showAboutDialog(application)
	})
	helpMenu.AddItem(aboutItem)

	menuBar.AddMenu(helpMenu)

	return menuBar
}

// createMainWindow creates the main demo window.
func createMainWindow(application *app.Application, statusBar *widgets.StatusBar) *window.Window {
	mainWindow := window.NewWindow("TUI Toolkit Demo")
	mainWindow.SetSize(core.UnitSize{
		Width:  core.Unit(60 * 8), // 60 columns
		Height: core.Unit(18 * 16), // 18 rows
	})

	// Create tab widget to organize demos
	tabWidget := widgets.NewTabWidget()

	// Add demo tabs
	tabWidget.AddTab("Basic Widgets", createBasicWidgetsDemo(statusBar))
	tabWidget.AddTab("Selection", createSelectionDemo())
	tabWidget.AddTab("Lists", createListDemo())
	tabWidget.AddTab("Progress", createProgressDemo())

	mainWindow.SetContent(tabWidget)

	return mainWindow
}

// createBasicWidgetsDemo creates a panel with basic widgets.
func createBasicWidgetsDemo(statusBar *widgets.StatusBar) core.Widget {
	panel := widgets.NewPanel()
	boxLayout := layout.NewBoxLayout(core.Vertical)
	boxLayout.SetSpacing(0)

	// Label
	label := widgets.NewLabel("This is a demo of basic widgets:")
	panel.AddChild(label)

	// Text input
	textInput := widgets.NewTextInput()
	textInput.SetPlaceholder("Enter text here...")
	textInput.SetOnTextChanged(func(text string) {
		if statusBar != nil {
			statusBar.SetText(fmt.Sprintf("Text: %s", text))
		}
	})
	panel.AddChild(textInput)

	// Buttons in a horizontal layout
	buttonPanel := widgets.NewPanel()
	hLayout := layout.NewBoxLayout(core.Horizontal)
	hLayout.SetSpacing(8)

	okButton := widgets.NewButton("OK")
	okButton.SetOnClick(func() {
		if statusBar != nil {
			statusBar.SetText("OK button clicked!")
		}
	})
	buttonPanel.AddChild(okButton)

	cancelButton := widgets.NewButton("Cancel")
	cancelButton.SetOnClick(func() {
		if statusBar != nil {
			statusBar.SetText("Cancel button clicked!")
		}
	})
	buttonPanel.AddChild(cancelButton)

	applyButton := widgets.NewButton("Apply")
	applyButton.SetOnClick(func() {
		if statusBar != nil {
			statusBar.SetText("Apply button clicked!")
		}
	})
	buttonPanel.AddChild(applyButton)

	buttonPanel.SetLayoutManager(hLayout)
	panel.AddChild(buttonPanel)

	// Disabled button
	disabledButton := widgets.NewButton("Disabled")
	disabledButton.SetEnabled(false)
	panel.AddChild(disabledButton)

	panel.SetLayoutManager(boxLayout)
	return panel
}

// createSelectionDemo creates a panel with selection widgets using a draggable splitter.
func createSelectionDemo() core.Widget {
	// Use a vertical splitter to divide checkboxes from radio buttons
	splitter := widgets.NewVSplitter()
	splitter.SetPosition(0.4) // Checkboxes get 40% of space

	// Top panel: Checkboxes
	checkPanel := widgets.NewPanel()
	checkLayout := layout.NewBoxLayout(core.Vertical)
	checkLayout.SetSpacing(0)

	checkLabel := widgets.NewLabel("Checkboxes:")
	checkPanel.AddChild(checkLabel)

	check1 := widgets.NewCheckbox("Enable feature A")
	check1.SetChecked(true)
	checkPanel.AddChild(check1)

	check2 := widgets.NewCheckbox("Enable feature B")
	checkPanel.AddChild(check2)

	check3 := widgets.NewCheckbox("Tri-state checkbox")
	check3.SetTriState(true)
	checkPanel.AddChild(check3)

	checkPanel.SetLayoutManager(checkLayout)
	splitter.SetFirst(checkPanel)

	// Bottom panel: Radio buttons and ComboBox
	radioPanel := widgets.NewPanel()
	radioLayout := layout.NewBoxLayout(core.Vertical)
	radioLayout.SetSpacing(0)

	radioLabel := widgets.NewLabel("Radio buttons:")
	radioPanel.AddChild(radioLabel)

	radioGroup := widgets.NewRadioGroup()
	radio1 := widgets.NewRadioButton("Option 1")
	radio2 := widgets.NewRadioButton("Option 2")
	radio3 := widgets.NewRadioButton("Option 3")
	radioGroup.AddButton(radio1)
	radioGroup.AddButton(radio2)
	radioGroup.AddButton(radio3)

	radioPanel.AddChild(radio1)
	radioPanel.AddChild(radio2)
	radioPanel.AddChild(radio3)

	// ComboBox
	comboLabel := widgets.NewLabel("ComboBox:")
	radioPanel.AddChild(comboLabel)

	combo := widgets.NewComboBox()
	combo.AddItem("First item")
	combo.AddItem("Second item")
	combo.AddItem("Third item")
	combo.AddItem("Fourth item")
	radioPanel.AddChild(combo)

	radioPanel.SetLayoutManager(radioLayout)
	splitter.SetSecond(radioPanel)

	return splitter
}

// createListDemo creates a panel with list widgets using a draggable splitter.
func createListDemo() core.Widget {
	// Use a horizontal splitter to divide space between ListView and TreeView
	splitter := widgets.NewHSplitter()
	splitter.SetPosition(0.5) // Start with 50/50 split

	// ListView
	listViewPanel := widgets.NewPanel()
	listLayout := layout.NewBoxLayout(core.Vertical)

	listLabel := widgets.NewLabel("ListView:")
	listViewPanel.AddChild(listLabel)

	listView := widgets.NewListView()
	for i := 1; i <= 20; i++ {
		item := widgets.NewListItem(fmt.Sprintf("Item %d", i))
		listView.AddItem(item)
	}
	listViewPanel.AddChild(listView)

	listViewPanel.SetLayoutManager(listLayout)
	splitter.SetFirst(listViewPanel)

	// TreeView
	treeViewPanel := widgets.NewPanel()
	treeLayout := layout.NewBoxLayout(core.Vertical)

	treeLabel := widgets.NewLabel("TreeView:")
	treeViewPanel.AddChild(treeLabel)

	treeView := widgets.NewTreeView()

	// Build tree structure
	root1 := widgets.NewTreeItem("Documents")
	root1.Expanded = true
	child1 := widgets.NewTreeItem("Work")
	child1.Expanded = true
	child1.AddChild(widgets.NewTreeItem("Report.txt"))
	child1.AddChild(widgets.NewTreeItem("Presentation.pptx"))
	root1.AddChild(child1)
	child2 := widgets.NewTreeItem("Personal")
	child2.AddChild(widgets.NewTreeItem("Notes.txt"))
	root1.AddChild(child2)

	root2 := widgets.NewTreeItem("Pictures")
	root2.AddChild(widgets.NewTreeItem("Vacation"))
	root2.AddChild(widgets.NewTreeItem("Family"))

	root3 := widgets.NewTreeItem("Downloads")

	treeView.AddRootItem(root1)
	treeView.AddRootItem(root2)
	treeView.AddRootItem(root3)

	treeViewPanel.AddChild(treeView)

	treeViewPanel.SetLayoutManager(treeLayout)
	splitter.SetSecond(treeViewPanel)

	return splitter
}

// createProgressDemo creates a panel with progress indicators.
func createProgressDemo() core.Widget {
	panel := widgets.NewPanel()
	boxLayout := layout.NewBoxLayout(core.Vertical)
	boxLayout.SetSpacing(16)

	// Horizontal progress bars
	hLabel := widgets.NewLabel("Horizontal Progress Bars:")
	panel.AddChild(hLabel)

	progress1 := widgets.NewProgressBar()
	progress1.SetValue(25)
	panel.AddChild(progress1)

	progress2 := widgets.NewProgressBar()
	progress2.SetValue(50)
	panel.AddChild(progress2)

	progress3 := widgets.NewProgressBar()
	progress3.SetValue(75)
	panel.AddChild(progress3)

	progress4 := widgets.NewProgressBar()
	progress4.SetValue(100)
	panel.AddChild(progress4)

	// Indeterminate progress
	indeterminateLabel := widgets.NewLabel("Indeterminate Progress:")
	panel.AddChild(indeterminateLabel)

	indeterminate := widgets.NewProgressBar()
	indeterminate.SetIndeterminate(true)
	panel.AddChild(indeterminate)

	panel.SetLayoutManager(boxLayout)
	return panel
}

// createDemoWindow creates a simple demo window with an embedded terminal.
func createDemoWindow(application *app.Application) *window.Window {
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

// showAboutDialog shows the about dialog.
func showAboutDialog(application *app.Application) {
	dialog := widgets.NewMessageBox(
		"About TUI Toolkit",
		"TUI Toolkit Demo\n\nA comprehensive terminal UI framework.\n\nVersion 0.1.0",
		widgets.ButtonOK,
	)
	dialog.SetIcon(widgets.IconInformation)
	dialog.SetOnFinished(func(result widgets.DialogResult) {
		// Dialog closed
	})

	// MessageBox is itself a window, just add it directly
	application.WindowManager().AddWindow(&dialog.Window)
}
