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
	d.mu.RLock()
	plat := d.platform
	surf := d.surface
	wm := d.windowManager
	d.mu.RUnlock()
	if plat == nil || surf == nil {
		return false
	}
	native, ok := surf.(platform.NativeSurface)
	if !ok {
		return false
	}
	gp, ok := plat.(platform.GlobalPointerPlatform)
	if !ok {
		return false
	}

	scale := d.deviceScale()
	deskX, deskY := native.ScreenPositionPx()
	b := win.Bounds()
	newSurf, err := plat.CreateSurface(platform.SurfaceOptions{
		Title:          win.Title(),
		Borderless:     true,
		CornerRadiusPx: int(window.FrameCornerRadius()) * scale,
		XPx:            deskX + int(e.X-offX)*scale,
		YPx:            deskY + int(e.Y-offY)*scale,
		WidthPx:        int(b.Width) * scale,
		HeightPx:       int(b.Height) * scale,
	})
	if err != nil {
		return false
	}

	wm.RemoveWindow(win)

	var host *window.TearOffHost
	host = window.NewTearOffHost(win, newSurf, scale, gp.GlobalPointerPx,
		func(gx, gy int, grabX, grabY core.Unit) bool {
			return d.redockAt(host, gx, gy, grabX, grabY)
		})
	// After a host-driven re-dock the torn surface lives on as an
	// invisible ghost, relaying the rest of its live mouse session
	// into the desktop's dispatch; the release closes it.
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

	// The [x] button on a torn window must take its OS window with it.
	host.SetOnClosed(func() {
		d.dropTornHost(host)
	})

	// Widgets in the torn window reach the platform clipboard through
	// the host (no desktop in their ancestry out there).
	host.SetClipboardAccess(d.Clipboard, d.SetClipboard)

	// Arm the host too: once the torn window exists under the held
	// pointer, the platform may hand it the rest of the gesture
	// (motion and the release) instead of the desktop. Whichever
	// window receives the stream drives the same drag.
	host.BeginDrag(offX, offY)

	d.mu.Lock()
	d.tornDrag = &tornDrag{host: host, surf: newSurf, offX: offX, offY: offY}
	d.tornHosts = append(d.tornHosts, host)
	d.mu.Unlock()
	return true
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

// redockAt serves a TearOffHost title drag: when the global pointer
// is over the desktop surface, reclaim the window there. Runs on the
// platform thread (host events).
func (d *Desktop) redockAt(host *window.TearOffHost, gx, gy int, grabX, grabY core.Unit) bool {
	d.mu.RLock()
	surf := d.surface
	d.mu.RUnlock()
	if surf == nil {
		return false
	}
	native, ok := surf.(platform.NativeSurface)
	if !ok {
		return false
	}
	if native.Minimized() {
		// A minimized desktop's rectangle is a phantom: dragging a
		// torn window across it must not dock into an invisible
		// desktop.
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
	// The TORN window owns this gesture's mouse session: keep its
	// surface alive (invisible) so the session can finish; the host
	// ghost-relays the rest of the drag into the desktop.
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
	d.windowManager.AddWindow(win)
	win.SetBounds(core.UnitRect{X: x, Y: y, Width: b.Width, Height: b.Height})
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
