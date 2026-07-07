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
		Title:      win.Title(),
		Borderless: true,
		XPx:        deskX + int(e.X-offX)*scale,
		YPx:        deskY + int(e.Y-offY)*scale,
		WidthPx:    int(b.Width) * scale,
		HeightPx:   int(b.Height) * scale,
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

	// Arm the host too: once the torn window exists under the held
	// pointer, the platform may hand it the rest of the gesture
	// (motion and the release) instead of the desktop. Whichever
	// window receives the stream drives the same drag.
	host.BeginDrag(offX, offY)

	d.mu.Lock()
	d.tornDrag = &tornDrag{host: host, surf: newSurf, offX: offX, offY: offY}
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
		size := surf.Size()
		if e.X >= 0 && e.Y >= 0 && e.X < size.Width && e.Y < size.Height {
			// Pointer came home: re-dock and hand the drag straight
			// back to the window manager.
			d.clearTornDrag(td)
			d.adoptTornWindow(td.host, e.X-td.offX, e.Y-td.offY)
			d.windowManager.BeginDrag(td.host.Window(), td.offX, td.offY)
			return true
		}
		if native, ok := td.surf.(platform.NativeSurface); ok {
			if deskNative, ok := surf.(platform.NativeSurface); ok {
				scale := d.deviceScale()
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
	scale := d.deviceScale()
	deskX, deskY := native.ScreenPositionPx()
	size := surf.Size()
	ux := core.Unit((gx - deskX) / scale)
	uy := core.Unit((gy - deskY) / scale)
	if ux < 0 || uy < 0 || ux >= size.Width || uy >= size.Height {
		return false
	}
	d.adoptTornWindow(host, ux-grabX, uy-grabY)
	d.windowManager.BeginDrag(host.Window(), grabX, grabY)
	return true
}

// adoptTornWindow closes the torn surface and puts the window back
// under the window manager at the given desktop-unit position.
func (d *Desktop) adoptTornWindow(host *window.TearOffHost, x, y core.Unit) {
	d.mu.Lock()
	if d.tornDrag != nil && d.tornDrag.host == host {
		d.tornDrag = nil
	}
	d.mu.Unlock()
	host.EndDrag()

	win := host.Window()
	b := win.Bounds()

	if native, ok := hostSurface(host).(platform.NativeSurface); ok {
		native.Close()
	}

	win.SetFlags(host.SavedFlags())
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
