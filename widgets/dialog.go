// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/window"
)

// DialogButton represents standard dialog buttons.
type DialogButton int

const (
	ButtonOK DialogButton = 1 << iota
	ButtonCancel
	ButtonYes
	ButtonNo
	ButtonRetry
	ButtonIgnore
	ButtonAbort
	ButtonSave
	ButtonDiscard
	ButtonApply
	ButtonHelp
)

// DialogResult represents the result of a dialog.
type DialogResult int

const (
	ResultNone DialogResult = iota
	ResultOK
	ResultCancel
	ResultYes
	ResultNo
	ResultRetry
	ResultIgnore
	ResultAbort
	ResultSave
	ResultDiscard
	ResultApply
	ResultHelp
)

// MessageBoxIcon represents message box icons.
type MessageBoxIcon int

const (
	IconNone MessageBoxIcon = iota
	IconInformation
	IconWarning
	IconError
	IconQuestion
)

// MessageBox displays a message with buttons.
type MessageBox struct {
	window.Window

	content *messageBoxContent
	buttons DialogButton
	result  DialogResult

	// Callbacks
	onFinished func(result DialogResult)
}

// messageBoxContent is the content widget for a MessageBox.
type messageBoxContent struct {
	core.WidgetBase
	icon          MessageBoxIcon
	text          string
	buttonWidgets []*Button
	onDone        func(result DialogResult)
}

// NewMessageBox creates a new message box.
func NewMessageBox(title, text string, buttons DialogButton) *MessageBox {
	m := &MessageBox{
		buttons: buttons,
		result:  ResultNone,
	}
	m.Window = *window.NewWindow(title)
	m.SetFlags(window.WindowFlagModal | window.WindowFlagNoResize)

	// Create content widget
	m.content = &messageBoxContent{
		text:   text,
		onDone: m.done,
	}
	m.content.WidgetBase = *core.NewWidgetBase()
	m.content.Init(m.content)
	m.content.SetFocusPolicy(core.StrongFocus)
	m.content.createButtons(buttons)

	// Set as window content
	m.SetContent(m.content)
	m.calculateSize()
	return m
}

// createButtons creates the dialog buttons for the content.
func (c *messageBoxContent) createButtons(buttons DialogButton) {
	buttonDefs := []struct {
		flag   DialogButton
		text   string
		result DialogResult
	}{
		{ButtonOK, "OK", ResultOK},
		{ButtonCancel, "Cancel", ResultCancel},
		{ButtonYes, "Yes", ResultYes},
		{ButtonNo, "No", ResultNo},
		{ButtonRetry, "Retry", ResultRetry},
		{ButtonIgnore, "Ignore", ResultIgnore},
		{ButtonAbort, "Abort", ResultAbort},
		{ButtonSave, "Save", ResultSave},
		{ButtonDiscard, "Discard", ResultDiscard},
		{ButtonApply, "Apply", ResultApply},
		{ButtonHelp, "Help", ResultHelp},
	}

	for _, def := range buttonDefs {
		if buttons&def.flag != 0 {
			btn := NewButton(def.text)
			result := def.result // Capture for closure
			btn.SetOnClick(func() {
				if c.onDone != nil {
					c.onDone(result)
				}
			})
			c.buttonWidgets = append(c.buttonWidgets, btn)
		}
	}
}

// calculateSize sets the dialog size based on content.
func (m *MessageBox) calculateSize() {
	metrics := core.DefaultCellMetrics()

	textWidth := len(m.content.text)
	if textWidth > 60 {
		textWidth = 60
	}
	if textWidth < 20 {
		textWidth = 20
	}

	m.SetBounds(core.UnitRect{
		Width:  core.Unit(textWidth+8) * metrics.CellWidth,
		Height: metrics.CellHeight * 8,
	})
}

// getIconText returns the text representation of the icon.
func (c *messageBoxContent) getIconText() string {
	switch c.icon {
	case IconInformation:
		return "ℹ"
	case IconWarning:
		return "⚠"
	case IconError:
		return "✖"
	case IconQuestion:
		return "?"
	default:
		return ""
	}
}

// SetIcon sets the message box icon.
func (m *MessageBox) SetIcon(icon MessageBoxIcon) {
	m.content.icon = icon
}

// SetOnFinished sets the finished callback.
func (m *MessageBox) SetOnFinished(handler func(result DialogResult)) {
	m.onFinished = handler
}

// Result returns the dialog result.
func (m *MessageBox) Result() DialogResult {
	return m.result
}

// done completes the dialog with the given result.
func (m *MessageBox) done(result DialogResult) {
	m.result = result
	if m.onFinished != nil {
		m.onFinished(result)
	}
	m.Close()
}

// Paint renders the message box content.
func (c *messageBoxContent) Paint(p *core.Painter) {
	bounds := c.Bounds()
	theme := c.Theme()
	metrics := p.Metrics()

	// Draw background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', theme.Normal)

	// Draw icon
	iconText := c.getIconText()
	textY := metrics.CellHeight
	if iconText != "" {
		p.DrawCell(metrics.CellWidth, textY, []rune(iconText)[0], theme.Normal)
	}

	// Draw message text
	textX := metrics.CellWidth * 4
	for _, ch := range c.text {
		if ch == '\n' {
			textY += metrics.CellHeight
			textX = metrics.CellWidth * 4
			continue
		}
		if textX >= bounds.Width-metrics.CellWidth {
			break
		}
		p.DrawCell(textX, textY, ch, theme.Normal)
		textX += metrics.CellWidth
	}

	// Draw buttons at bottom
	buttonY := bounds.Height - metrics.CellHeight*2
	buttonX := metrics.CellWidth

	for _, btn := range c.buttonWidgets {
		btnWidth := core.Unit(len(btn.Text())+4) * metrics.CellWidth
		btn.SetBounds(core.UnitRect{
			X:      buttonX,
			Y:      buttonY,
			Width:  btnWidth,
			Height: metrics.CellHeight,
		})
		// Use a translated painter for the button at its position
		btnPainter := p.WithOffset(buttonX, buttonY)
		btn.Paint(btnPainter)
		buttonX += btnWidth + metrics.CellWidth
	}
}

// HandleMousePress handles mouse clicks on buttons.
func (c *messageBoxContent) HandleMousePress(event core.MousePressEvent) bool {
	// Check if click is on any button
	for _, btn := range c.buttonWidgets {
		btnBounds := btn.Bounds()
		if event.X >= btnBounds.X && event.X < btnBounds.X+btnBounds.Width &&
			event.Y >= btnBounds.Y && event.Y < btnBounds.Y+btnBounds.Height {
			// Translate event to button's local coordinates
			localEvent := event
			localEvent.X -= btnBounds.X
			localEvent.Y -= btnBounds.Y
			return btn.HandleMousePress(localEvent)
		}
	}
	return false
}

// HandleMouseRelease handles mouse release on buttons.
func (c *messageBoxContent) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	// Forward to all buttons (the pressed one will handle it)
	for _, btn := range c.buttonWidgets {
		btnBounds := btn.Bounds()
		localEvent := event
		localEvent.X -= btnBounds.X
		localEvent.Y -= btnBounds.Y
		if btn.HandleMouseRelease(localEvent) {
			return true
		}
	}
	return false
}

// HandleKeyPress handles keyboard input.
func (m *MessageBox) HandleKeyPress(event core.KeyPressEvent) bool {
	switch event.Key {
	case "Escape":
		if m.buttons&ButtonCancel != 0 {
			m.done(ResultCancel)
		} else if m.buttons&ButtonNo != 0 {
			m.done(ResultNo)
		}
		return true

	case "Enter":
		if m.buttons&ButtonOK != 0 {
			m.done(ResultOK)
		} else if m.buttons&ButtonYes != 0 {
			m.done(ResultYes)
		}
		return true
	}

	return m.Window.HandleKeyPress(event)
}

// Information shows an information message box.
func Information(title, text string) DialogResult {
	mb := NewMessageBox(title, text, ButtonOK)
	mb.SetIcon(IconInformation)
	return ResultOK
}

// Warning shows a warning message box.
func Warning(title, text string) DialogResult {
	mb := NewMessageBox(title, text, ButtonOK)
	mb.SetIcon(IconWarning)
	return ResultOK
}

// ErrorDialog shows an error message box.
func ErrorDialog(title, text string) DialogResult {
	mb := NewMessageBox(title, text, ButtonOK)
	mb.SetIcon(IconError)
	return ResultOK
}

// Question shows a question message box.
func Question(title, text string) DialogResult {
	mb := NewMessageBox(title, text, ButtonYes|ButtonNo)
	mb.SetIcon(IconQuestion)
	return ResultYes
}

// FileDialogMode represents the file dialog mode.
type FileDialogMode int

const (
	FileDialogOpen FileDialogMode = iota
	FileDialogSave
	FileDialogSelectDirectory
)

// FileFilter represents a file filter.
type FileFilter struct {
	Name     string   // e.g., "Text Files"
	Patterns []string // e.g., ["*.txt", "*.text"]
}

// FileDialog provides file selection functionality.
type FileDialog struct {
	window.Window

	mode          FileDialogMode
	directory     string
	fileName      string
	filters       []FileFilter
	currentFilter int

	// State
	entries      []os.DirEntry
	currentIndex int
	scrollOffset int

	// Widgets
	pathInput     *TextInput
	fileList      *ListView
	fileNameInput *TextInput
	filterCombo   *ComboBox
	okButton      *Button
	cancelButton  *Button

	// Callbacks
	onFileSelected func(path string)
	onCancelled    func()
}

// NewFileDialog creates a new file dialog.
func NewFileDialog(mode FileDialogMode) *FileDialog {
	f := &FileDialog{
		mode:      mode,
		directory: ".",
	}

	title := "Open"
	switch mode {
	case FileDialogSave:
		title = "Save As"
	case FileDialogSelectDirectory:
		title = "Select Directory"
	}

	f.Window = *window.NewWindow(title)
	f.SetFlags(window.WindowFlagModal)
	f.setupUI()
	return f
}

// setupUI creates the file dialog UI.
func (f *FileDialog) setupUI() {
	metrics := core.DefaultCellMetrics()

	// Path input
	f.pathInput = NewTextInput()
	f.pathInput.SetText(f.directory)
	f.pathInput.SetOnReturnPressed(func() {
		f.navigateTo(f.pathInput.Text())
	})

	// File list
	f.fileList = NewListView()
	f.fileList.SetOnItemActivated(func(index int) {
		f.itemActivated(index)
	})
	f.fileList.SetOnCurrentChanged(func(index int) {
		if index >= 0 && index < len(f.entries) {
			entry := f.entries[index]
			if entry != nil && !entry.IsDir() && f.fileNameInput != nil {
				f.fileNameInput.SetText(entry.Name())
			}
		}
	})

	// File name input (for save dialog)
	if f.mode == FileDialogSave {
		f.fileNameInput = NewTextInput()
		f.fileNameInput.SetPlaceholder("Enter file name")
	}

	// Filter combo
	f.filterCombo = NewComboBox()

	// Buttons
	buttonText := "Open"
	if f.mode == FileDialogSave {
		buttonText = "Save"
	} else if f.mode == FileDialogSelectDirectory {
		buttonText = "Select"
	}

	f.okButton = NewButton(buttonText)
	f.okButton.SetOnClick(func() { f.accept() })

	f.cancelButton = NewButton("Cancel")
	f.cancelButton.SetOnClick(func() { f.reject() })

	// Set size
	f.SetBounds(core.UnitRect{
		Width:  metrics.CellWidth * 60,
		Height: metrics.CellHeight * 20,
	})

	// Initial load
	f.refreshFileList()
}

// SetDirectory sets the initial directory.
func (f *FileDialog) SetDirectory(dir string) {
	f.directory = dir
	if f.pathInput != nil {
		f.pathInput.SetText(dir)
	}
	f.refreshFileList()
}

// SetFileName sets the initial file name.
func (f *FileDialog) SetFileName(name string) {
	f.fileName = name
	if f.fileNameInput != nil {
		f.fileNameInput.SetText(name)
	}
}

// AddFilter adds a file filter.
func (f *FileDialog) AddFilter(name string, patterns ...string) {
	f.filters = append(f.filters, FileFilter{
		Name:     name,
		Patterns: patterns,
	})
	if f.filterCombo != nil {
		f.filterCombo.AddItem(name)
	}
}

// SetOnFileSelected sets the file selected callback.
func (f *FileDialog) SetOnFileSelected(handler func(path string)) {
	f.onFileSelected = handler
}

// SetOnCancelled sets the cancelled callback.
func (f *FileDialog) SetOnCancelled(handler func()) {
	f.onCancelled = handler
}

// SelectedFile returns the selected file path.
func (f *FileDialog) SelectedFile() string {
	return filepath.Join(f.directory, f.fileName)
}

// refreshFileList refreshes the file list.
func (f *FileDialog) refreshFileList() {
	f.fileList.Clear()
	f.entries = nil

	// Read directory
	entries, err := os.ReadDir(f.directory)
	if err != nil {
		f.fileList.AddTextItem("Error: " + err.Error())
		return
	}

	// Add parent directory
	if f.directory != "/" && f.directory != "." {
		item := NewListItem("..")
		f.fileList.AddItem(item)
		f.entries = append(f.entries, nil) // Placeholder for parent
	}

	// Sort: directories first, then files
	var dirs, files []os.DirEntry
	for _, entry := range entries {
		// Skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			// Apply filter
			if f.matchesFilter(entry.Name()) {
				files = append(files, entry)
			}
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name()) < strings.ToLower(dirs[j].Name())
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name()) < strings.ToLower(files[j].Name())
	})

	// Add directories
	for _, entry := range dirs {
		item := NewListItem("[DIR] " + entry.Name())
		f.fileList.AddItem(item)
		f.entries = append(f.entries, entry)
	}

	// Add files (skip for directory selection)
	if f.mode != FileDialogSelectDirectory {
		for _, entry := range files {
			item := NewListItem("     " + entry.Name())
			f.fileList.AddItem(item)
			f.entries = append(f.entries, entry)
		}
	}
}

// matchesFilter checks if a file matches the current filter.
func (f *FileDialog) matchesFilter(name string) bool {
	if len(f.filters) == 0 || f.currentFilter < 0 || f.currentFilter >= len(f.filters) {
		return true
	}

	filter := f.filters[f.currentFilter]
	for _, pattern := range filter.Patterns {
		if pattern == "*.*" || pattern == "*" {
			return true
		}
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
	}
	return false
}

// navigateTo navigates to a directory.
func (f *FileDialog) navigateTo(path string) {
	// Handle relative paths
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.directory, path)
	}

	// Clean the path
	path = filepath.Clean(path)

	// Check if it exists and is a directory
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return
	}

	f.directory = path
	f.pathInput.SetText(path)
	f.refreshFileList()
}

// itemActivated handles item activation.
func (f *FileDialog) itemActivated(index int) {
	if index < 0 || index >= len(f.entries) {
		return
	}

	entry := f.entries[index]

	// Parent directory
	if entry == nil {
		f.navigateTo(filepath.Dir(f.directory))
		return
	}

	if entry.IsDir() {
		f.navigateTo(filepath.Join(f.directory, entry.Name()))
	} else {
		f.fileName = entry.Name()
		if f.fileNameInput != nil {
			f.fileNameInput.SetText(entry.Name())
		}
		f.accept()
	}
}

// accept accepts the dialog.
func (f *FileDialog) accept() {
	if f.mode == FileDialogSave && f.fileNameInput != nil {
		f.fileName = f.fileNameInput.Text()
	}

	if f.mode == FileDialogSelectDirectory {
		f.fileName = ""
	}

	if f.onFileSelected != nil {
		f.onFileSelected(f.SelectedFile())
	}
	f.Close()
}

// reject rejects the dialog.
func (f *FileDialog) reject() {
	if f.onCancelled != nil {
		f.onCancelled()
	}
	f.Close()
}

// Paint renders the file dialog.
func (f *FileDialog) Paint(p *core.Painter) {
	// First paint the window frame
	f.Window.Paint(p)

	bounds := f.Bounds()
	metrics := p.Metrics()

	// Layout widgets manually
	y := metrics.CellHeight * 2

	// Path label and input
	p.DrawText(metrics.CellWidth*2, y, "Location:", f.Theme().Normal)
	f.pathInput.SetBounds(core.UnitRect{
		X:      metrics.CellWidth * 12,
		Y:      y,
		Width:  bounds.Width - metrics.CellWidth*14,
		Height: metrics.CellHeight,
	})
	f.pathInput.Paint(p)
	y += metrics.CellHeight + metrics.CellHeight/2

	// File list
	listHeight := bounds.Height - metrics.CellHeight*8
	f.fileList.SetBounds(core.UnitRect{
		X:      metrics.CellWidth * 2,
		Y:      y,
		Width:  bounds.Width - metrics.CellWidth*4,
		Height: listHeight,
	})
	f.fileList.Paint(p)
	y += listHeight + metrics.CellHeight/2

	// File name input (for save dialog)
	if f.fileNameInput != nil {
		p.DrawText(metrics.CellWidth*2, y, "File name:", f.Theme().Normal)
		f.fileNameInput.SetBounds(core.UnitRect{
			X:      metrics.CellWidth * 14,
			Y:      y,
			Width:  bounds.Width - metrics.CellWidth*16,
			Height: metrics.CellHeight,
		})
		f.fileNameInput.Paint(p)
		y += metrics.CellHeight + metrics.CellHeight/2
	}

	// Buttons at bottom
	buttonY := bounds.Height - metrics.CellHeight*2
	buttonWidth := metrics.CellWidth * 10

	f.okButton.SetBounds(core.UnitRect{
		X:      bounds.Width - buttonWidth*2 - metrics.CellWidth*4,
		Y:      buttonY,
		Width:  buttonWidth,
		Height: metrics.CellHeight,
	})
	f.okButton.Paint(p)

	f.cancelButton.SetBounds(core.UnitRect{
		X:      bounds.Width - buttonWidth - metrics.CellWidth*2,
		Y:      buttonY,
		Width:  buttonWidth,
		Height: metrics.CellHeight,
	})
	f.cancelButton.Paint(p)
}

// HandleKeyPress handles keyboard input.
func (f *FileDialog) HandleKeyPress(event core.KeyPressEvent) bool {
	switch event.Key {
	case "Escape":
		f.reject()
		return true

	case "Enter":
		if f.fileList.HasFocus() && f.fileList.CurrentIndex() >= 0 {
			f.itemActivated(f.fileList.CurrentIndex())
			return true
		}
		f.accept()
		return true

	case "Backspace":
		if !f.pathInput.HasFocus() && (f.fileNameInput == nil || !f.fileNameInput.HasFocus()) {
			f.navigateTo(filepath.Dir(f.directory))
			return true
		}
	}

	return f.Window.HandleKeyPress(event)
}

// OpenFile shows an open file dialog.
func OpenFile(filters ...FileFilter) string {
	dialog := NewFileDialog(FileDialogOpen)
	for _, filter := range filters {
		dialog.AddFilter(filter.Name, filter.Patterns...)
	}
	return ""
}

// SaveFile shows a save file dialog.
func SaveFile(defaultName string, filters ...FileFilter) string {
	dialog := NewFileDialog(FileDialogSave)
	dialog.SetFileName(defaultName)
	for _, filter := range filters {
		dialog.AddFilter(filter.Name, filter.Patterns...)
	}
	return ""
}

// SelectDirectory shows a directory selection dialog.
func SelectDirectory(startDir string) string {
	dialog := NewFileDialog(FileDialogSelectDirectory)
	dialog.SetDirectory(startDir)
	return ""
}

// InputDialog shows a simple input dialog.
type InputDialog struct {
	window.Window

	labelText    string
	input        *TextInput
	okButton     *Button
	cancelButton *Button

	result   string
	accepted bool

	onFinished func(text string, accepted bool)
}

// NewInputDialog creates a new input dialog.
func NewInputDialog(title, label, defaultValue string) *InputDialog {
	d := &InputDialog{
		labelText: label,
	}
	d.Window = *window.NewWindow(title)
	d.SetFlags(window.WindowFlagModal | window.WindowFlagNoResize)

	metrics := core.DefaultCellMetrics()

	d.input = NewTextInput()
	d.input.SetText(defaultValue)
	d.input.SelectAll()

	d.okButton = NewButton("OK")
	d.okButton.SetOnClick(func() {
		d.result = d.input.Text()
		d.accepted = true
		if d.onFinished != nil {
			d.onFinished(d.result, true)
		}
		d.Close()
	})

	d.cancelButton = NewButton("Cancel")
	d.cancelButton.SetOnClick(func() {
		d.accepted = false
		if d.onFinished != nil {
			d.onFinished("", false)
		}
		d.Close()
	})

	d.SetBounds(core.UnitRect{
		Width:  metrics.CellWidth * 40,
		Height: metrics.CellHeight * 6,
	})

	return d
}

// Result returns the input result.
func (d *InputDialog) Result() string {
	return d.result
}

// Accepted returns whether OK was pressed.
func (d *InputDialog) Accepted() bool {
	return d.accepted
}

// SetOnFinished sets the finished callback.
func (d *InputDialog) SetOnFinished(handler func(text string, accepted bool)) {
	d.onFinished = handler
}

// Paint renders the input dialog.
func (d *InputDialog) Paint(p *core.Painter) {
	d.Window.Paint(p)

	bounds := d.Bounds()
	metrics := p.Metrics()
	theme := d.Theme()

	// Label
	y := metrics.CellHeight * 2
	p.DrawText(metrics.CellWidth*2, y, d.labelText, theme.Normal)

	// Input
	y += metrics.CellHeight
	d.input.SetBounds(core.UnitRect{
		X:      metrics.CellWidth * 2,
		Y:      y,
		Width:  bounds.Width - metrics.CellWidth*4,
		Height: metrics.CellHeight,
	})
	d.input.Paint(p)

	// Buttons
	buttonY := bounds.Height - metrics.CellHeight*2
	buttonWidth := metrics.CellWidth * 10

	d.okButton.SetBounds(core.UnitRect{
		X:      bounds.Width/2 - buttonWidth - metrics.CellWidth,
		Y:      buttonY,
		Width:  buttonWidth,
		Height: metrics.CellHeight,
	})
	d.okButton.Paint(p)

	d.cancelButton.SetBounds(core.UnitRect{
		X:      bounds.Width/2 + metrics.CellWidth,
		Y:      buttonY,
		Width:  buttonWidth,
		Height: metrics.CellHeight,
	})
	d.cancelButton.Paint(p)
}

// HandleKeyPress handles keyboard input.
func (d *InputDialog) HandleKeyPress(event core.KeyPressEvent) bool {
	switch event.Key {
	case "Escape":
		d.accepted = false
		if d.onFinished != nil {
			d.onFinished("", false)
		}
		d.Close()
		return true

	case "Enter":
		d.result = d.input.Text()
		d.accepted = true
		if d.onFinished != nil {
			d.onFinished(d.result, true)
		}
		d.Close()
		return true
	}

	return d.Window.HandleKeyPress(event)
}

// GetText shows an input dialog and returns the text.
func GetText(title, label, defaultValue string) (string, bool) {
	_ = NewInputDialog(title, label, defaultValue)
	return "", false
}
