// Package main demonstrates the TUI toolkit capabilities.
package main

import (
	"fmt"

	"github.com/phroun/tuitk/app"
	"github.com/phroun/tuitk/backend"
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/layout"
	"github.com/phroun/tuitk/style"
	"github.com/phroun/tuitk/widgets"
	"github.com/phroun/tuitk/window"
)

func main() {
	// Create the TUI backend with default options
	opts := backend.DefaultTUIOptions()
	tuiBackend := backend.NewTUIBackend(opts)

	// Create the application
	application := app.New(tuiBackend)

	// Create desktop with Mac-style menu bar at top
	desktop := widgets.NewDesktop()

	// Create the application menu bar (at desktop level, not per-window)
	menuBar := createMenuBar(application)
	desktop.SetMenuBar(menuBar)

	// Create a status bar at the bottom
	statusBar := widgets.NewStatusBar()
	// Use styled text to highlight keyboard shortcuts in red
	redStyle := style.DefaultStyle().WithFg(style.ColorRed).WithBg(style.ColorWhite)
	statusBar.SetStyledText([]widgets.StatusTextSpan{
		{Text: "Ready - Press "},
		{Text: "F10", Style: &redStyle},
		{Text: " for menu, Tab to navigate, "},
		{Text: "Ctrl+Q", Style: &redStyle},
		{Text: " to quit"},
	})
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
	fileMenu := widgets.NewMenu("&File")
	newItem := widgets.NewMenuItem("&New")
	newItem.SetShortcut(core.NewShortcut("^N"))
	fileMenu.AddItem(newItem)

	openItem := widgets.NewMenuItem("&Open...")
	openItem.SetShortcut(core.NewShortcut("^O"))
	fileMenu.AddItem(openItem)

	saveItem := widgets.NewMenuItem("&Save")
	saveItem.SetShortcut(core.NewShortcut("^S"))
	fileMenu.AddItem(saveItem)

	fileMenu.AddSeparator()

	exitItem := widgets.NewMenuItem("E&xit")
	exitItem.SetShortcut(core.NewShortcut("^Q"))
	exitItem.SetOnTriggered(func() {
		application.Quit()
	})
	fileMenu.AddItem(exitItem)

	menuBar.AddMenu(fileMenu)

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

	menuBar.AddMenu(editMenu)

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

	menuBar.AddMenu(viewMenu)

	// Window menu
	windowMenu := widgets.NewMenu("&Window")

	newWindowItem := widgets.NewMenuItem("&New Window")
	newWindowItem.SetOnTriggered(func() {
		demoWindow := createDemoWindow(application)
		application.WindowManager().AddWindow(demoWindow)
	})
	windowMenu.AddItem(newWindowItem)

	windowMenu.AddSeparator()

	tileItem := widgets.NewMenuItem("&Tile")
	tileItem.SetOnTriggered(func() {
		application.WindowManager().TileWindows()
	})
	windowMenu.AddItem(tileItem)

	cascadeItem := widgets.NewMenuItem("&Cascade")
	cascadeItem.SetOnTriggered(func() {
		application.WindowManager().CascadeWindows()
	})
	windowMenu.AddItem(cascadeItem)

	menuBar.AddMenu(windowMenu)

	// Help menu
	helpMenu := widgets.NewMenu("&Help")

	aboutItem := widgets.NewMenuItem("&About")
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
	tabWidget.AddTab("Selection", createSelectionDemo(tabWidget))
	tabWidget.AddTab("Lists", createListDemo())
	tabWidget.AddTab("Scroll Selection", createScrollSelectionDemo(tabWidget))
	tabWidget.AddTab("Scroll Lists", createScrollListDemo())
	tabWidget.AddTab("Progress", createProgressDemo())
	tabWidget.AddTab("Bottom Tabs", createBottomTabsDemo())

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

	// Add a spacer before buttons
	spacer := widgets.NewSpacer()
	panel.AddChild(spacer)

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
func createSelectionDemo(tabWidget *widgets.TabWidget) core.Widget {
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

	// Background color radio buttons
	bgLabel := widgets.NewLabel("Tab Background Color:")
	radioPanel.AddChild(bgLabel)

	bgGroup := widgets.NewRadioGroup()
	bgDefault := widgets.NewRadioButton("Default")
	bgGreen := widgets.NewRadioButton("Dark Green")
	bgGray := widgets.NewRadioButton("TrueColor #333")
	bgGroup.AddButton(bgDefault)
	bgGroup.AddButton(bgGreen)
	bgGroup.AddButton(bgGray)
	bgDefault.SetChecked(true)

	// Set up callbacks to change TabWidget background
	bgDefault.SetOnToggled(func(checked bool) {
		if checked {
			tabWidget.SetBackgroundColor(nil) // Inherit/default
			tabWidget.Update()
		}
	})
	bgGreen.SetOnToggled(func(checked bool) {
		if checked {
			green := style.ColorGreen
			tabWidget.SetBackgroundColor(&green)
			tabWidget.Update()
		}
	})
	bgGray.SetOnToggled(func(checked bool) {
		if checked {
			gray := style.RGB(0x33, 0x33, 0x33)
			tabWidget.SetBackgroundColor(&gray)
			tabWidget.Update()
		}
	})

	radioPanel.AddChild(bgDefault)
	radioPanel.AddChild(bgGreen)
	radioPanel.AddChild(bgGray)

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
	child1.AddChild(widgets.NewTreeItem("Budget.xlsx"))
	child1.AddChild(widgets.NewTreeItem("Meeting Notes.md"))
	root1.AddChild(child1)
	child2 := widgets.NewTreeItem("Personal")
	child2.AddChild(widgets.NewTreeItem("Notes.txt"))
	child2.AddChild(widgets.NewTreeItem("Journal.md"))
	child2.AddChild(widgets.NewTreeItem("Ideas.txt"))
	root1.AddChild(child2)
	child3 := widgets.NewTreeItem("Projects")
	child3.AddChild(widgets.NewTreeItem("Alpha"))
	child3.AddChild(widgets.NewTreeItem("Beta"))
	child3.AddChild(widgets.NewTreeItem("Gamma"))
	root1.AddChild(child3)

	root2 := widgets.NewTreeItem("Pictures")
	root2.AddChild(widgets.NewTreeItem("Vacation"))
	root2.AddChild(widgets.NewTreeItem("Family"))
	root2.AddChild(widgets.NewTreeItem("Pets"))
	root2.AddChild(widgets.NewTreeItem("Events"))
	root2.AddChild(widgets.NewTreeItem("Screenshots"))

	root3 := widgets.NewTreeItem("Downloads")
	root3.AddChild(widgets.NewTreeItem("Software"))
	root3.AddChild(widgets.NewTreeItem("Documents"))
	root3.AddChild(widgets.NewTreeItem("Music"))

	root4 := widgets.NewTreeItem("Music")
	root4.AddChild(widgets.NewTreeItem("Rock"))
	root4.AddChild(widgets.NewTreeItem("Jazz"))
	root4.AddChild(widgets.NewTreeItem("Classical"))
	root4.AddChild(widgets.NewTreeItem("Electronic"))

	root5 := widgets.NewTreeItem("Videos")
	root5.AddChild(widgets.NewTreeItem("Movies"))
	root5.AddChild(widgets.NewTreeItem("TV Shows"))
	root5.AddChild(widgets.NewTreeItem("Tutorials"))

	root6 := widgets.NewTreeItem("Code")
	codeChild1 := widgets.NewTreeItem("Go")
	codeChild1.AddChild(widgets.NewTreeItem("main.go"))
	codeChild1.AddChild(widgets.NewTreeItem("utils.go"))
	root6.AddChild(codeChild1)
	codeChild2 := widgets.NewTreeItem("Python")
	codeChild2.AddChild(widgets.NewTreeItem("script.py"))
	root6.AddChild(codeChild2)

	treeView.AddRootItem(root1)
	treeView.AddRootItem(root2)
	treeView.AddRootItem(root3)
	treeView.AddRootItem(root4)
	treeView.AddRootItem(root5)
	treeView.AddRootItem(root6)

	treeViewPanel.AddChild(treeView)

	treeViewPanel.SetLayoutManager(treeLayout)
	splitter.SetSecond(treeViewPanel)

	return splitter
}

// createScrollSelectionDemo creates a panel with selection widgets wrapped in scroll areas.
func createScrollSelectionDemo(tabWidget *widgets.TabWidget) core.Widget {
	// Use a vertical splitter to divide checkboxes from radio buttons
	splitter := widgets.NewVSplitter()
	splitter.SetPosition(0.4) // Checkboxes get 40% of space

	// Top panel: Checkboxes (wrapped in scroll area)
	checkPanel := widgets.NewPanel()
	checkLayout := layout.NewBoxLayout(core.Vertical)
	checkLayout.SetSpacing(0)

	checkLabel := widgets.NewLabel("Checkboxes (scrollable):")
	checkPanel.AddChild(checkLabel)

	// Add more checkboxes to demonstrate scrolling
	for i := 1; i <= 15; i++ {
		check := widgets.NewCheckbox(fmt.Sprintf("Feature option %d", i))
		if i%3 == 0 {
			check.SetChecked(true)
		}
		checkPanel.AddChild(check)
	}

	checkPanel.SetLayoutManager(checkLayout)

	// Wrap in scroll area
	checkScroll := widgets.NewScrollArea()
	checkScroll.SetContent(checkPanel)
	checkScroll.SetVerticalScrollBarPolicy(widgets.ScrollBarAsNeeded)
	checkScroll.SetHorizontalScrollBarPolicy(widgets.ScrollBarAsNeeded)
	splitter.SetFirst(checkScroll)

	// Bottom panel: Radio buttons and ComboBox (wrapped in scroll area)
	radioPanel := widgets.NewPanel()
	radioLayout := layout.NewBoxLayout(core.Vertical)
	radioLayout.SetSpacing(0)

	radioLabel := widgets.NewLabel("Radio buttons (scrollable):")
	radioPanel.AddChild(radioLabel)

	radioGroup := widgets.NewRadioGroup()
	for i := 1; i <= 10; i++ {
		radio := widgets.NewRadioButton(fmt.Sprintf("Radio option %d with longer text", i))
		radioGroup.AddButton(radio)
		radioPanel.AddChild(radio)
	}

	// Background color radio buttons
	bgLabel := widgets.NewLabel("Tab Background Color:")
	radioPanel.AddChild(bgLabel)

	bgGroup := widgets.NewRadioGroup()
	bgDefault := widgets.NewRadioButton("Default")
	bgGreen := widgets.NewRadioButton("Dark Green")
	bgGray := widgets.NewRadioButton("TrueColor #333")
	bgGroup.AddButton(bgDefault)
	bgGroup.AddButton(bgGreen)
	bgGroup.AddButton(bgGray)
	bgDefault.SetChecked(true)

	// Set up callbacks to change TabWidget background
	bgDefault.SetOnToggled(func(checked bool) {
		if checked {
			tabWidget.SetBackgroundColor(nil) // Inherit/default
			tabWidget.Update()
		}
	})
	bgGreen.SetOnToggled(func(checked bool) {
		if checked {
			green := style.ColorGreen
			tabWidget.SetBackgroundColor(&green)
			tabWidget.Update()
		}
	})
	bgGray.SetOnToggled(func(checked bool) {
		if checked {
			gray := style.RGB(0x33, 0x33, 0x33)
			tabWidget.SetBackgroundColor(&gray)
			tabWidget.Update()
		}
	})

	radioPanel.AddChild(bgDefault)
	radioPanel.AddChild(bgGreen)
	radioPanel.AddChild(bgGray)

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

	// Wrap in scroll area
	radioScroll := widgets.NewScrollArea()
	radioScroll.SetContent(radioPanel)
	radioScroll.SetVerticalScrollBarPolicy(widgets.ScrollBarAsNeeded)
	radioScroll.SetHorizontalScrollBarPolicy(widgets.ScrollBarAsNeeded)
	splitter.SetSecond(radioScroll)

	return splitter
}

// createScrollListDemo creates a panel with list widgets wrapped in scroll areas.
func createScrollListDemo() core.Widget {
	// Use a horizontal splitter to divide space between ListView and TreeView
	splitter := widgets.NewHSplitter()
	splitter.SetPosition(0.5) // Start with 50/50 split

	// ListView (wrapped in scroll area)
	listViewPanel := widgets.NewPanel()
	listLayout := layout.NewBoxLayout(core.Vertical)

	listLabel := widgets.NewLabel("ListView (scrollable container):")
	listViewPanel.AddChild(listLabel)

	listView := widgets.NewListView()
	for i := 1; i <= 20; i++ {
		item := widgets.NewListItem(fmt.Sprintf("Item %d", i))
		listView.AddItem(item)
	}
	listViewPanel.AddChild(listView)

	// Add some extra widgets to make the panel taller than the view
	extraLabel := widgets.NewLabel("Extra content below ListView:")
	listViewPanel.AddChild(extraLabel)

	for i := 1; i <= 5; i++ {
		btn := widgets.NewButton(fmt.Sprintf("Button %d", i))
		listViewPanel.AddChild(btn)
	}

	listViewPanel.SetLayoutManager(listLayout)

	// Wrap in scroll area
	listScroll := widgets.NewScrollArea()
	listScroll.SetContent(listViewPanel)
	listScroll.SetVerticalScrollBarPolicy(widgets.ScrollBarAsNeeded)
	listScroll.SetHorizontalScrollBarPolicy(widgets.ScrollBarAsNeeded)
	splitter.SetFirst(listScroll)

	// TreeView (wrapped in scroll area)
	treeViewPanel := widgets.NewPanel()
	treeLayout := layout.NewBoxLayout(core.Vertical)

	treeLabel := widgets.NewLabel("TreeView (scrollable container):")
	treeViewPanel.AddChild(treeLabel)

	treeView := widgets.NewTreeView()

	// Build tree structure
	root1 := widgets.NewTreeItem("Documents")
	root1.Expanded = true
	child1 := widgets.NewTreeItem("Work")
	child1.Expanded = true
	child1.AddChild(widgets.NewTreeItem("Report.txt"))
	child1.AddChild(widgets.NewTreeItem("Presentation.pptx"))
	child1.AddChild(widgets.NewTreeItem("Budget.xlsx"))
	child1.AddChild(widgets.NewTreeItem("Meeting Notes.md"))
	root1.AddChild(child1)
	child2 := widgets.NewTreeItem("Personal")
	child2.AddChild(widgets.NewTreeItem("Notes.txt"))
	child2.AddChild(widgets.NewTreeItem("Journal.md"))
	child2.AddChild(widgets.NewTreeItem("Ideas.txt"))
	root1.AddChild(child2)
	child3 := widgets.NewTreeItem("Projects")
	child3.AddChild(widgets.NewTreeItem("Alpha"))
	child3.AddChild(widgets.NewTreeItem("Beta"))
	child3.AddChild(widgets.NewTreeItem("Gamma"))
	root1.AddChild(child3)

	root2 := widgets.NewTreeItem("Pictures")
	root2.AddChild(widgets.NewTreeItem("Vacation"))
	root2.AddChild(widgets.NewTreeItem("Family"))
	root2.AddChild(widgets.NewTreeItem("Pets"))
	root2.AddChild(widgets.NewTreeItem("Events"))
	root2.AddChild(widgets.NewTreeItem("Screenshots"))

	root3 := widgets.NewTreeItem("Downloads")
	root3.AddChild(widgets.NewTreeItem("Software"))
	root3.AddChild(widgets.NewTreeItem("Documents"))
	root3.AddChild(widgets.NewTreeItem("Music"))

	root4 := widgets.NewTreeItem("Music")
	root4.AddChild(widgets.NewTreeItem("Rock"))
	root4.AddChild(widgets.NewTreeItem("Jazz"))
	root4.AddChild(widgets.NewTreeItem("Classical"))
	root4.AddChild(widgets.NewTreeItem("Electronic"))

	root5 := widgets.NewTreeItem("Videos")
	root5.AddChild(widgets.NewTreeItem("Movies"))
	root5.AddChild(widgets.NewTreeItem("TV Shows"))
	root5.AddChild(widgets.NewTreeItem("Tutorials"))

	root6 := widgets.NewTreeItem("Code")
	codeChild1 := widgets.NewTreeItem("Go")
	codeChild1.AddChild(widgets.NewTreeItem("main.go"))
	codeChild1.AddChild(widgets.NewTreeItem("utils.go"))
	root6.AddChild(codeChild1)
	codeChild2 := widgets.NewTreeItem("Python")
	codeChild2.AddChild(widgets.NewTreeItem("script.py"))
	root6.AddChild(codeChild2)

	treeView.AddRootItem(root1)
	treeView.AddRootItem(root2)
	treeView.AddRootItem(root3)
	treeView.AddRootItem(root4)
	treeView.AddRootItem(root5)
	treeView.AddRootItem(root6)

	treeViewPanel.AddChild(treeView)

	// Add extra content below TreeView
	extraLabel2 := widgets.NewLabel("Extra content below TreeView:")
	treeViewPanel.AddChild(extraLabel2)

	textInput := widgets.NewTextInput()
	textInput.SetPlaceholder("Type something...")
	treeViewPanel.AddChild(textInput)

	treeViewPanel.SetLayoutManager(treeLayout)

	// Wrap in scroll area
	treeScroll := widgets.NewScrollArea()
	treeScroll.SetContent(treeViewPanel)
	treeScroll.SetVerticalScrollBarPolicy(widgets.ScrollBarAsNeeded)
	treeScroll.SetHorizontalScrollBarPolicy(widgets.ScrollBarAsNeeded)
	splitter.SetSecond(treeScroll)

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

// createBottomTabsDemo creates a panel with a nested TabWidget using bottom tabs.
func createBottomTabsDemo() core.Widget {
	// Create a TabWidget with tabs at the bottom
	bottomTabs := widgets.NewTabWidget()
	bottomTabs.SetTabPosition(widgets.TabsBottom)

	// Add some content tabs
	tab1Panel := widgets.NewPanel()
	tab1Layout := layout.NewBoxLayout(core.Vertical)
	tab1Label := widgets.NewLabel("This TabWidget has tabs at the bottom.")
	tab1Panel.AddChild(tab1Label)
	tab1Desc := widgets.NewLabel("Notice how the tab connectors are inverted:")
	tab1Panel.AddChild(tab1Desc)
	tab1Example := widgets.NewLabel("  Top tabs use: _/ and \\_")
	tab1Panel.AddChild(tab1Example)
	tab1Example2 := widgets.NewLabel("  Bottom tabs use: \\_ and _/")
	tab1Panel.AddChild(tab1Example2)
	tab1Panel.SetLayoutManager(tab1Layout)
	bottomTabs.AddTab("First", tab1Panel)

	tab2Panel := widgets.NewPanel()
	tab2Layout := layout.NewBoxLayout(core.Vertical)
	tab2Label := widgets.NewLabel("Second tab content")
	tab2Panel.AddChild(tab2Label)
	tab2Button := widgets.NewButton("Click me")
	tab2Panel.AddChild(tab2Button)
	tab2Panel.SetLayoutManager(tab2Layout)
	bottomTabs.AddTab("Second", tab2Panel)

	tab3Panel := widgets.NewPanel()
	tab3Layout := layout.NewBoxLayout(core.Vertical)
	tab3Label := widgets.NewLabel("Third tab with an input field:")
	tab3Panel.AddChild(tab3Label)
	tab3Input := widgets.NewTextInput()
	tab3Input.SetPlaceholder("Type here...")
	tab3Panel.AddChild(tab3Input)
	tab3Panel.SetLayoutManager(tab3Layout)
	bottomTabs.AddTab("Third", tab3Panel)

	return bottomTabs
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
