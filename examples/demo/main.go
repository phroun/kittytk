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
	application.SetStatusBarContent([]widgets.StatusSection{
		{Spans: []widgets.StatusTextSpan{
			{Text: "Ready - Press "},
			{Text: "F10", Style: &redStyle},
			{Text: " for menu, Tab to navigate, "},
			{Text: "Ctrl+Q", Style: &redStyle},
			{Text: " to quit"},
		}},
	})

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

// createMainWindow creates the main demo window.
func createMainWindow(desktop *widgets.Desktop, application *app.Application) *window.Window {
	mainWindow := window.NewWindow("TUI Toolkit Demo")
	mainWindow.SetSize(core.UnitSize{
		Width:  core.Unit(60 * 8), // 60 columns
		Height: core.Unit(18 * 16), // 18 rows
	})

	// Create tab widget to organize demos
	tabWidget := widgets.NewTabWidget()

	// Add demo tabs
	tabWidget.AddTab("Basic Widgets", createBasicWidgetsDemo(desktop))
	tabWidget.AddTab("Selection", createSelectionDemo(tabWidget))
	tabWidget.AddTab("Lists", createListDemo())
	tabWidget.AddTab("Scroll Selection", createScrollSelectionDemo(tabWidget))
	tabWidget.AddTab("Scroll Lists", createScrollListDemo())
	tabWidget.AddTab("Progress", createProgressDemo())
	tabWidget.AddTab("Bottom Tabs", createBottomTabsDemo())
	tabWidget.AddTab("Vertical Tabs", createVerticalTabsDemo())
	tabWidget.AddTab("MDI Demo", createMDIDemo(desktop, application, mainWindow))

	mainWindow.SetContent(tabWidget)

	return mainWindow
}

// createBasicWidgetsDemo creates a panel with basic widgets.
func createBasicWidgetsDemo(desktop *widgets.Desktop) core.Widget {
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
		if statusBar := desktop.StatusBar(); statusBar != nil {
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
		if sb := desktop.StatusBar(); sb != nil {
			sb.SetText("OK button clicked!")
		}
	})
	buttonPanel.AddChild(okButton)

	cancelButton := widgets.NewButton("Cancel")
	cancelButton.SetOnClick(func() {
		if sb := desktop.StatusBar(); sb != nil {
			sb.SetText("Cancel button clicked!")
		}
	})
	buttonPanel.AddChild(cancelButton)

	applyButton := widgets.NewButton("Apply")
	applyButton.SetOnClick(func() {
		if sb := desktop.StatusBar(); sb != nil {
			sb.SetText("Apply button clicked!")
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

	// Alphabet ComboBox (26 items for testing scrolling)
	alphabetLabel := widgets.NewLabel("Alphabet ComboBox:")
	radioPanel.AddChild(alphabetLabel)

	alphabetCombo := widgets.NewComboBox()
	for i := 0; i < 26; i++ {
		letter := string(rune('A' + i))
		alphabetCombo.AddItem(letter + " - Letter " + letter)
	}
	radioPanel.AddChild(alphabetCombo)

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

// createVerticalTabsDemo creates a panel with vertical tabs demonstration.
func createVerticalTabsDemo() core.Widget {
	// Use a horizontal splitter to show left and right tab widgets side by side
	splitter := widgets.NewHSplitter()
	splitter.SetPosition(0.5)

	// Left: TabWidget with tabs on the left
	leftTabWidget := widgets.NewTabWidget()
	leftTabWidget.SetTabPosition(widgets.TabsLeft)

	leftTab1 := widgets.NewPanel()
	leftTab1Layout := layout.NewBoxLayout(core.Vertical)
	leftTab1Label := widgets.NewLabel("This is the first tab in a\nTabsLeft layout.")
	leftTab1.AddChild(leftTab1Label)
	leftTab1Desc := widgets.NewLabel("Tabs are displayed vertically\nalong the left edge.")
	leftTab1.AddChild(leftTab1Desc)
	leftTab1.SetLayoutManager(leftTab1Layout)
	leftTabWidget.AddTab("First", leftTab1)

	leftTab2 := widgets.NewPanel()
	leftTab2Layout := layout.NewBoxLayout(core.Vertical)
	leftTab2Label := widgets.NewLabel("Second tab content")
	leftTab2.AddChild(leftTab2Label)
	leftTab2Button := widgets.NewButton("A Button")
	leftTab2.AddChild(leftTab2Button)
	leftTab2.SetLayoutManager(leftTab2Layout)
	leftTabWidget.AddTab("Second", leftTab2)

	leftTab3 := widgets.NewPanel()
	leftTab3Layout := layout.NewBoxLayout(core.Vertical)
	leftTab3Input := widgets.NewTextInput()
	leftTab3Input.SetPlaceholder("Type here...")
	leftTab3.AddChild(leftTab3Input)
	leftTab3.SetLayoutManager(leftTab3Layout)
	leftTabWidget.AddTab("Third", leftTab3)

	splitter.SetFirst(leftTabWidget)

	// Right: TabWidget with tabs on the right
	rightTabWidget := widgets.NewTabWidget()
	rightTabWidget.SetTabPosition(widgets.TabsRight)

	rightTab1 := widgets.NewPanel()
	rightTab1Layout := layout.NewBoxLayout(core.Vertical)
	rightTab1Label := widgets.NewLabel("This is the first tab in a\nTabsRight layout.")
	rightTab1.AddChild(rightTab1Label)
	rightTab1Desc := widgets.NewLabel("Tabs are displayed vertically\nalong the right edge.")
	rightTab1.AddChild(rightTab1Desc)
	rightTab1.SetLayoutManager(rightTab1Layout)
	rightTabWidget.AddTab("Alpha", rightTab1)

	rightTab2 := widgets.NewPanel()
	rightTab2Layout := layout.NewBoxLayout(core.Vertical)
	rightTab2Label := widgets.NewLabel("Beta tab content")
	rightTab2.AddChild(rightTab2Label)
	rightTab2Check := widgets.NewCheckbox("Enable option")
	rightTab2.AddChild(rightTab2Check)
	rightTab2.SetLayoutManager(rightTab2Layout)
	rightTabWidget.AddTab("Beta", rightTab2)

	rightTab3 := widgets.NewPanel()
	rightTab3Layout := layout.NewBoxLayout(core.Vertical)
	rightTab3Label := widgets.NewLabel("Gamma tab content")
	rightTab3.AddChild(rightTab3Label)
	rightTab3.SetLayoutManager(rightTab3Layout)
	rightTabWidget.AddTab("Gamma", rightTab3)

	splitter.SetSecond(rightTabWidget)

	return splitter
}

// MDI child window counter for unique naming
var mdiChildCount int

// createMDIDemo creates a panel demonstrating MDI-style child windows.
func createMDIDemo(desktop *widgets.Desktop, application *app.Application, parentWindow *window.Window) core.Widget {
	panel := widgets.NewPanel()
	boxLayout := layout.NewBoxLayout(core.Vertical)
	boxLayout.SetSpacing(8)

	// Description
	descLabel := widgets.NewLabel("MDI (Multiple Document Interface) Demo")
	panel.AddChild(descLabel)

	infoLabel := widgets.NewLabel("Click the button below to spawn new child windows.\nChild windows appear within the parent window and can be\nmoved, resized, minimized, and maximized.")
	panel.AddChild(infoLabel)

	// Button to spawn new MDI child window
	spawnButton := widgets.NewButton("Spawn MDI Child Window")
	spawnButton.SetOnClick(func() {
		mdiChildCount++
		childWindow := createMDIChildWindow(parentWindow, mdiChildCount)
		childWindow.SetParentWindow(parentWindow)
	})
	panel.AddChild(spawnButton)

	// Add a spacer
	spacer := widgets.NewSpacer()
	panel.AddChild(spacer)

	// Info about window management
	tipLabel := widgets.NewLabel("Tips:")
	panel.AddChild(tipLabel)

	tip1 := widgets.NewLabel("- Drag title bar to move windows")
	panel.AddChild(tip1)

	tip2 := widgets.NewLabel("- Drag edges to resize windows")
	panel.AddChild(tip2)

	tip3 := widgets.NewLabel("- Click [_] to minimize to dock")
	panel.AddChild(tip3)

	tip4 := widgets.NewLabel("- Click [#] to maximize window")
	panel.AddChild(tip4)

	tip5 := widgets.NewLabel("- Use Window > Tile or Cascade")
	panel.AddChild(tip5)

	panel.SetLayoutManager(boxLayout)
	return panel
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
		Width:  core.Unit(45 * 8),
		Height: core.Unit(12 * 16),
	})

	// Create simple content
	panel := widgets.NewPanel()
	boxLayout := layout.NewBoxLayout(core.Vertical)
	boxLayout.SetSpacing(8)

	label := widgets.NewLabel(fmt.Sprintf("This window belongs to Application #%d", appNum))
	panel.AddChild(label)

	infoLabel := widgets.NewLabel("Notice the menu bar and status bar change\nwhen this window is focused.")
	panel.AddChild(infoLabel)

	textInput := widgets.NewTextInput()
	textInput.SetPlaceholder("Enter text here...")
	panel.AddChild(textInput)

	closeButton := widgets.NewButton("Close Window")
	closeButton.SetOnClick(func() {
		w.Close()
	})
	panel.AddChild(closeButton)

	panel.SetLayoutManager(boxLayout)
	w.SetContent(panel)

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
