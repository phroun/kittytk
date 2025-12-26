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
						if focused != nil {
							focusInfo = fmt.Sprintf("focused: %T (chain: %d)", focused, len(chain))
						} else {
							focusInfo = fmt.Sprintf("focused: nil (chain: %d)", len(chain))
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
	boxLayout.SetSpacing(8)

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

// createSelectionDemo creates a panel with selection widgets.
func createSelectionDemo() core.Widget {
	panel := widgets.NewPanel()
	boxLayout := layout.NewBoxLayout(core.Vertical)
	boxLayout.SetSpacing(8)

	// Checkboxes
	checkLabel := widgets.NewLabel("Checkboxes:")
	panel.AddChild(checkLabel)

	check1 := widgets.NewCheckbox("Enable feature A")
	check1.SetChecked(true)
	panel.AddChild(check1)

	check2 := widgets.NewCheckbox("Enable feature B")
	panel.AddChild(check2)

	check3 := widgets.NewCheckbox("Tri-state checkbox")
	check3.SetTriState(true)
	panel.AddChild(check3)

	// Radio buttons
	radioLabel := widgets.NewLabel("Radio buttons:")
	panel.AddChild(radioLabel)

	radioGroup := widgets.NewRadioGroup()
	radio1 := widgets.NewRadioButton("Option 1")
	radio2 := widgets.NewRadioButton("Option 2")
	radio3 := widgets.NewRadioButton("Option 3")
	radioGroup.AddButton(radio1)
	radioGroup.AddButton(radio2)
	radioGroup.AddButton(radio3)

	panel.AddChild(radio1)
	panel.AddChild(radio2)
	panel.AddChild(radio3)

	// ComboBox
	comboLabel := widgets.NewLabel("ComboBox:")
	panel.AddChild(comboLabel)

	combo := widgets.NewComboBox()
	combo.AddItem("First item")
	combo.AddItem("Second item")
	combo.AddItem("Third item")
	combo.AddItem("Fourth item")
	panel.AddChild(combo)

	panel.SetLayoutManager(boxLayout)
	return panel
}

// createListDemo creates a panel with list widgets.
func createListDemo() core.Widget {
	panel := widgets.NewPanel()
	hLayout := layout.NewBoxLayout(core.Horizontal)
	hLayout.SetSpacing(8)

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
	panel.AddChild(listViewPanel)

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
	panel.AddChild(treeViewPanel)

	panel.SetLayoutManager(hLayout)
	return panel
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

// createDemoWindow creates a simple demo window.
func createDemoWindow(application *app.Application) *window.Window {
	w := window.NewWindow("Demo Window")
	w.SetSize(core.UnitSize{
		Width:  core.Unit(40 * 8),
		Height: core.Unit(12 * 16),
	})

	panel := widgets.NewPanel()
	boxLayout := layout.NewBoxLayout(core.Vertical)
	boxLayout.SetSpacing(8)

	label := widgets.NewLabel("This is a child window.")
	panel.AddChild(label)

	textInput := widgets.NewTextInput()
	textInput.SetPlaceholder("Type something...")
	panel.AddChild(textInput)

	closeButton := widgets.NewButton("Close")
	closeButton.SetOnClick(func() {
		w.Close()
	})
	panel.AddChild(closeButton)

	panel.SetLayoutManager(boxLayout)
	w.SetContent(panel)

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
