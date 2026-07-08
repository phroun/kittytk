package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/platform"
	"github.com/phroun/tuitk/window"
)

// Tear-off choreography (G4 granting, desktop side). On multi-surface
// platforms a desktop window dragged past the surface edge undocks
// into its own borderless OS window - chrome intact, so it looks the
// same torn or docked - and re-docks when the pointer crosses back
// over the desktop mid-drag.
//
// Two drags exist. While the tearing gesture is live the DESKTOP
// window still owns the pointer capture, so the desktop keeps
// receiving motion and drives the torn surface itself (tornDrag). A
// later drag started on the torn window's own title bar is driven by
// its TearOffHost, which asks the desktop via the redock callback
// whether the pointer has come home.

// tornDrag is the desktop-driven phase of a tear-off: the gesture
// that tore the window is still holding the button.
type tornDrag struct {
	host *window.TearOffHost
	surf platform.Surface
	offX core.Unit // grab offset within the window, units
	offY core.Unit
}

// setupTearOff arms the window manager's tear-off policy when the
// platform can host more than one surface.
func (d *Desktop) setupTearOff(p platform.Platform, surf platform.Surface) {
	ms, ok := p.(platform.MultiSurfacePlatform)
	if !ok || !ms.SupportsMultipleSurfaces() {
		return
	}
	if _, ok := surf.(platform.NativeSurface); !ok {
		return
	}
	if _, ok := p.(platform.GlobalPointerPlatform); !ok {
		return
	}
	d.windowManager.SetTearOffHandler(d.tearOffWindow)
}

// deviceScale is the desktop surface's pixels-per-unit.
func (d *Desktop) deviceScale() int {
	d.mu.RLock()
	backend := d.backend
	d.mu.RUnlock()
	if ds, ok := backend.(core.DeviceScaler); ok {
		if s := ds.Scale(); s > 0 {
			return s
		}
	}
	return 1
}

// tearOffWindow implements the WindowManager tear-off policy: lift
// the window out into its own OS surface positioned so the grab
// point stays under the pointer, and keep driving the drag from the
// desktop (which still owns the capture).
func (d *Desktop) tearOffWindow(win *window.Window, e core.MouseMoveEvent, offX, offY core.Unit) bool {
	// Only tearable windows detach - dialogs and other plain windows
	// stay put.
	if !win.IsTearable() {
		return false
	}
	host := d.createTornHost(win, e.X-offX, e.Y-offY)
	if host == nil {
		return false
	}
	// Drag path: keep driving the gesture from the desktop's captured
	// pointer stream until release.
	host.BeginDrag(offX, offY)
	d.mu.Lock()
	d.tornDrag = &tornDrag{host: host, surf: host.Surface(), offX: offX, offY: offY}
	d.mu.Unlock()
	return true
}

// tearOffInPlace detaches a docked tearable window into its own
// surface at its current desktop position and size, without a drag -
// the tear-handle click/keyboard activation path.
func (d *Desktop) tearOffInPlace(win *window.Window) {
	if !win.IsTearable() || win.IsDetached() {
		return
	}
	b := win.Bounds()
	d.createTornHost(win, b.X, b.Y)
}

// createTornHost lifts win out of the window manager into a new
// borderless OS surface whose top-left is at the given desktop-unit
// position. Returns nil when the platform can't host it. Shared by
// the drag and click detach paths.
func (d *Desktop) createTornHost(win *window.Window, deskUnitX, deskUnitY core.Unit) *window.TearOffHost {
	d.mu.RLock()
	plat := d.platform
	surf := d.surface
	wm := d.windowManager
	d.mu.RUnlock()
	if plat == nil || surf == nil {
		return nil
	}
	native, ok := surf.(platform.NativeSurface)
	if !ok {
		return nil
	}
	gp, ok := plat.(platform.GlobalPointerPlatform)
	if !ok {
		return nil
	}

	scale := d.deviceScale()
	deskX, deskY := native.ScreenPositionPx()
	b := win.Bounds()
	newSurf, err := plat.CreateSurface(platform.SurfaceOptions{
		Title:          win.Title(),
		Borderless:     true,
		CornerRadiusPx: int(window.FrameCornerRadius()) * scale,
		XPx:            deskX + int(deskUnitX)*scale,
		YPx:            deskY + int(deskUnitY)*scale,
		WidthPx:        int(b.Width) * scale,
		HeightPx:       int(b.Height) * scale,
	})
	if err != nil {
		return nil
	}

	wm.RemoveWindow(win)

	var host *window.TearOffHost
	// A detached window re-docks by dragging its '#' handle back over
	// the desktop, or by clicking it. The host only calls this during
	// a HANDLE drag - a plain title drag just moves the OS window.
	host = window.NewTearOffHost(win, newSurf, scale, gp.GlobalPointerPx,
		func(gx, gy int, grabX, grabY core.Unit) bool {
			return d.redockAt(host, gx, gy, grabX, grabY)
		})
	host.SetGhostRelay(
		func(gx, gy int) {
			ux, uy := d.globalToDesktopUnits(gx, gy)
			d.dispatchEvent(core.MouseMoveEvent{X: ux, Y: uy, Buttons: core.LeftButton})
			d.invalidateSurface()
		},
		func() {
			gx, gy := gp.GlobalPointerPx()
			ux, uy := d.globalToDesktopUnits(gx, gy)
			d.dispatchEvent(core.MouseReleaseEvent{X: ux, Y: uy, Button: core.LeftButton})
			if native, ok := newSurf.(platform.NativeSurface); ok {
				native.Close()
			}
			d.invalidateSurface()
		})
	host.SetOnClosed(func() { d.dropTornHost(host) })
	host.SetClipboardAccess(d.Clipboard, d.SetClipboard)

	// A torn window still borrows the desktop's menu bar line: when its
	// surface gains focus, point the menu bar at this window's app so the
	// app's menus are actually reachable (they showed nowhere before).
	host.SetOnFocus(func(focused bool) {
		if focused {
			d.windowFocusChanged(win)
			d.invalidateSurface()
		}
	})

	// The window now reads as detached (handle shows '#'); clicking
	// the handle (or Cmd-style activation) re-docks it to the desktop.
	win.SetDetached(true)
	// A detached main window carries the app's own menu bar + status bar.
	d.attachMainWindowChrome(win)
	win.SetOnTearRequest(func() { d.redockInPlace(host) })

	d.mu.Lock()
	d.tornHosts = append(d.tornHosts, host)
	d.mu.Unlock()
	return host
}

// redockInPlace re-docks a torn window to the desktop at its current
// on-screen position, retaining its size - the '#' handle click path.
func (d *Desktop) redockInPlace(host *window.TearOffHost) {
	d.mu.RLock()
	surf := d.surface
	d.mu.RUnlock()
	deskNative, ok := surf.(platform.NativeSurface)
	if !ok {
		return
	}
	tornNative, ok := host.Surface().(platform.NativeSurface)
	if !ok {
		return
	}
	scale := d.deviceScale()
	deskX, deskY := deskNative.ScreenPositionPx()
	tx, ty := tornNative.ScreenPositionPx()
	ux := core.Unit((tx - deskX) / scale)
	uy := core.Unit((ty - deskY) / scale)
	d.adoptTornWindow(host, ux, uy, false)
}

// handleTornDrag continues a live tear gesture from the desktop's
// event stream (the desktop window still owns the capture). Returns
// false when no tear drag is active so normal dispatch proceeds.
func (d *Desktop) handleTornDrag(event core.Event) bool {
	d.mu.RLock()
	td := d.tornDrag
	surf := d.surface
	d.mu.RUnlock()
	if td == nil {
		return false
	}

	switch e := event.(type) {
	case core.MousePressEvent:
		// A fresh press means the tearing gesture ended somewhere we
		// couldn't see (the torn window took the release). Stale
		// state: the window stays torn, the press proceeds normally.
		d.clearTornDrag(td)
		return false

	case core.MouseMoveEvent:
		if e.Buttons&core.LeftButton == 0 {
			// Button no longer held: the release went to the torn
			// window. The gesture is over; do NOT re-dock on a mere
			// hover.
			d.clearTornDrag(td)
			return false
		}
		// Position from the GLOBAL pointer, not the event: when OS
		// mouse capture is lost mid-gesture (window churn can drop
		// it), SDL clamps window-relative motion to the window rect,
		// which would fence the torn window into a small range around
		// the desktop. The events remain the ticks; the global
		// pointer is the truth.
		ux, uy := e.X, e.Y
		gx, gy := 0, 0
		haveGlobal := false
		d.mu.RLock()
		plat := d.platform
		d.mu.RUnlock()
		if gp, ok := plat.(platform.GlobalPointerPlatform); ok {
			gx, gy = gp.GlobalPointerPx()
			ux, uy = d.globalToDesktopUnits(gx, gy)
			haveGlobal = true
		}
		size := surf.Size()
		if ux >= 0 && uy >= 0 && ux < size.Width && uy < size.Height {
			// Pointer came home: re-dock and hand the drag straight
			// back to the window manager.
			d.clearTornDrag(td)
			// The desktop owns this gesture's mouse session, so the
			// torn surface can be destroyed immediately.
			d.adoptTornWindow(td.host, ux-td.offX, uy-td.offY, false)
			d.windowManager.BeginDrag(td.host.Window(), td.offX, td.offY)
			return true
		}
		if native, ok := td.surf.(platform.NativeSurface); ok {
			scale := d.deviceScale()
			if haveGlobal {
				native.SetScreenPositionPx(gx-int(td.offX)*scale, gy-int(td.offY)*scale)
			} else if deskNative, ok := surf.(platform.NativeSurface); ok {
				deskX, deskY := deskNative.ScreenPositionPx()
				native.SetScreenPositionPx(
					deskX+int(e.X-td.offX)*scale,
					deskY+int(e.Y-td.offY)*scale)
			}
		}
		return true

	case core.MouseReleaseEvent:
		// Dropped outside: the window stays torn off; its host owns
		// any further drags.
		_ = e
		d.clearTornDrag(td)
		return true
	}
	return false
}

// clearTornDrag ends the desktop-driven tear phase and disarms the
// host's mirror of the same gesture.
func (d *Desktop) clearTornDrag(td *tornDrag) {
	d.mu.Lock()
	if d.tornDrag == td {
		d.tornDrag = nil
	}
	d.mu.Unlock()
	td.host.EndDrag()
}

// redockAt serves a TearOffHost handle drag: when the global pointer
// is over the desktop surface, reclaim the window there (retaining
// size), enforcing the reachability bounds. The torn surface stays
// alive as a ghost until its live mouse session finishes.
func (d *Desktop) redockAt(host *window.TearOffHost, gx, gy int, grabX, grabY core.Unit) bool {
	d.mu.RLock()
	surf := d.surface
	d.mu.RUnlock()
	native, ok := surf.(platform.NativeSurface)
	if !ok || native.Minimized() {
		return false
	}
	scale := d.deviceScale()
	deskX, deskY := native.ScreenPositionPx()
	size := surf.Size()
	ux := core.Unit((gx - deskX) / scale)
	uy := core.Unit((gy - deskY) / scale)
	if ux < 0 || uy < 0 || ux >= size.Width || uy >= size.Height {
		return false
	}
	d.adoptTornWindow(host, ux-grabX, uy-grabY, true)
	d.windowManager.BeginDrag(host.Window(), grabX, grabY)
	return true
}

// dropTornHost disposes of a torn window's surface and forgets the
// host (the window closed itself while torn).
func (d *Desktop) dropTornHost(host *window.TearOffHost) {
	d.mu.Lock()
	if d.tornDrag != nil && d.tornDrag.host == host {
		d.tornDrag = nil
	}
	// A closing torn window can't keep owning focus/the menu bar line.
	if d.tornFocusOwner == host.Window() {
		d.tornFocusOwner = nil
	}
	for i, th := range d.tornHosts {
		if th == host {
			d.tornHosts = append(d.tornHosts[:i], d.tornHosts[i+1:]...)
			break
		}
	}
	d.mu.Unlock()
	if native, ok := host.Surface().(platform.NativeSurface); ok {
		native.Close()
	}
	d.invalidateSurface()
}

// globalToDesktopUnits converts a global pixel position to desktop
// surface units.
func (d *Desktop) globalToDesktopUnits(gx, gy int) (core.Unit, core.Unit) {
	d.mu.RLock()
	surf := d.surface
	d.mu.RUnlock()
	native, ok := surf.(platform.NativeSurface)
	if !ok {
		return 0, 0
	}
	scale := d.deviceScale()
	deskX, deskY := native.ScreenPositionPx()
	return core.Unit((gx - deskX) / scale), core.Unit((gy - deskY) / scale)
}

// invalidateSurface requests a desktop repaint.
func (d *Desktop) invalidateSurface() {
	d.mu.RLock()
	surf := d.surface
	d.mu.RUnlock()
	if surf != nil {
		surf.Invalidate(core.UnitRect{})
	}
}

// adoptTornWindow puts the window back under the window manager at
// the given desktop-unit position. ghost keeps the torn surface
// alive but invisible (its mouse session must finish before it can
// be destroyed); otherwise it closes immediately.
func (d *Desktop) adoptTornWindow(host *window.TearOffHost, x, y core.Unit, ghost bool) {
	d.mu.Lock()
	if d.tornDrag != nil && d.tornDrag.host == host {
		d.tornDrag = nil
	}
	for i, th := range d.tornHosts {
		if th == host {
			d.tornHosts = append(d.tornHosts[:i], d.tornHosts[i+1:]...)
			break
		}
	}
	d.mu.Unlock()
	host.EndDrag()

	win := host.Window()
	b := win.Bounds()

	if native, ok := hostSurface(host).(platform.NativeSurface); ok {
		if ghost {
			native.SetOpacity(0)
		} else {
			native.Close()
		}
	}

	win.SetFlags(host.SavedFlags())
	win.SetOnBoundsRequest(nil)
	// Re-docked: the handle reads '%' again and its click re-tears.
	win.SetDetached(false)
	// Docked again: the app's menus return to the desktop bar.
	d.detachMainWindowChrome(win)
	win.SetOnTearRequest(func() { d.tearOffInPlace(win) })
	d.windowManager.AddWindow(win)
	// Keep the re-docked window reachable: title bar within the client
	// area, a couple of columns visible horizontally.
	win.SetBounds(d.windowManager.ClampToClientArea(core.UnitRect{X: x, Y: y, Width: b.Width, Height: b.Height}))
	win.Layout()
	d.windowManager.ActivateWindow(win)

	d.mu.RLock()
	surf := d.surface
	d.mu.RUnlock()
	if surf != nil {
		surf.Invalidate(core.UnitRect{})
	}
}

// hostSurface exposes the host's surface for teardown.
func hostSurface(h *window.TearOffHost) platform.Surface { return h.Surface() }
