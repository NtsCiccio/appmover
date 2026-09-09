//go:build windows

package tray

import (
	"appmover/internal/applog"
	"appmover/internal/autostart"
	"appmover/internal/config"
	"appmover/internal/movewindow"
	"appmover/internal/msgloop"
	"appmover/internal/state"
	"appmover/internal/vdesktop"
	"appmover/internal/win32"
	"appmover/internal/winlist"
	_ "embed"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"fyne.io/systray"
	w32 "github.com/gonutz/w32/v2"
)

//go:embed icon.ico
var iconData []byte

const (
	hotkeyNext = 1
	hotkeyPrev = 2
)

// mu guards every field below that's read or written from more than one
// goroutine: monitors/monitorLabels (rebuilt on WM_DISPLAYCHANGE) and each
// slot's targets/checkedMonitor (written by refreshWindows, read by the
// per-submenu click goroutines).
var mu sync.RWMutex

var (
	quitItem      *systray.MenuItem
	autostartItem *systray.MenuItem
	moreItem      *systray.MenuItem // "+N more windows not shown"

	monitors      []win32.Monitor
	monitorLabels []string

	slots []*windowSlot

	cfg    config.Config
	st     *state.State
	vdm    *vdesktop.Manager // nil if virtual-desktop filtering is unavailable
	loop   *msgloop.Loop     // nil if the event-driven message loop couldn't start
	logger *log.Logger
)

// slotTarget is the window a menu slot currently represents, for one
// specific monitor's submenu entry.
type slotTarget struct {
	handle w32.HWND
	pid    uint32
}

// windowSlot is a reusable menu entry: instead of destroying and
// recreating entries on every refresh (which would leave goroutines
// hanging on the ClickedCh of removed entries), we update the title and
// targets of existing entries and hide them when not needed. Only a rare
// monitor-configuration change (WM_DISPLAYCHANGE) rebuilds the per-monitor
// submenus themselves.
type windowSlot struct {
	item     *systray.MenuItem
	subItems []*systray.MenuItem
	targets  []slotTarget // same order as 'monitors'

	lastTitle      string
	visible        bool
	checkedMonitor int // index into subItems currently checked ("last used"), -1 if none
}

func initLogger() {
	l, err := applog.Open()
	if err != nil {
		// Nowhere left to log to: AppMover builds with -H=windowsgui (no
		// console), so stderr would be a silent no-op anyway. logger stays
		// nil and logInfo becomes a no-op.
		return
	}
	logger = l
}

func logInfo(format string, args ...any) {
	if logger != nil {
		logger.Printf(format, args...)
	}
}

// OnReady is called once the tray has been initialized.
func OnReady() {
	initLogger()
	logInfo("OnReady: started")

	if c, err := config.Load(); err != nil {
		logInfo("config.Load: %v (using defaults)", err)
		cfg = config.Default()
	} else {
		cfg = c
	}

	if s, err := state.Load(); err != nil {
		logInfo("state.Load: %v (starting fresh)", err)
		st = s
	} else {
		st = s
	}

	systray.SetIcon(iconData)
	systray.SetTitle("AppMover")
	systray.SetTooltip("Move apps between monitors")

	mu.Lock()
	monitors = win32.EnumMonitors()
	monitorLabels = computeMonitorLabels(monitors)
	mu.Unlock()
	logInfo("OnReady: found %d monitors", len(monitors))

	if v, err := vdesktop.New(); err != nil {
		logInfo("vdesktop.New: %v (virtual-desktop filtering disabled)", err)
	} else {
		vdm = v
	}

	buildSlotPool(cfg.MaxWindowSlots)

	systray.AddSeparator()

	enabled, err := autostart.IsEnabled()
	if err != nil {
		logInfo("autostart.IsEnabled: %v", err)
	}
	autostartItem = systray.AddMenuItemCheckbox("Start with Windows", "Automatically start AppMover at login", enabled)
	go handleAutostartToggle()

	quitItem = systray.AddMenuItem("Quit", "Quit the application")

	if l, err := msgloop.Start(); err != nil {
		logInfo("msgloop.Start: %v (hotkeys and instant refresh disabled, falling back to polling only)", err)
	} else {
		loop = l
		registerHotkeys()
		go handleLoopEvents()
		go handleHotkeys()
	}

	go handleQuit()
	go refreshLoop()
	refreshWindows()
}

// OnExit is invoked when systray.Quit() is called.
func OnExit() {
	logInfo("OnExit: called")
	if loop != nil {
		loop.Close()
	}
	if vdm != nil {
		vdm.Close()
	}
	if st != nil {
		if err := st.Save(); err != nil {
			logInfo("state.Save: %v", err)
		}
	}
}

func computeMonitorLabels(monitors []win32.Monitor) []string {
	labels := make([]string, len(monitors))
	for i, m := range monitors {
		label := fmt.Sprintf("Monitor (%dx%d)", m.Bounds.Width(), m.Bounds.Height())
		if m.IsPrimary {
			label += " – primary"
		}
		labels[i] = label
	}
	return labels
}

// buildSlotPool pre-creates n window-menu entries plus the trailing
// "+N more" indicator. Created only once, then only updated or
// hidden/shown — never destroyed (see windowSlot).
func buildSlotPool(n int) {
	mu.Lock()
	defer mu.Unlock()

	for i := 0; i < n; i++ {
		item := systray.AddMenuItem("", "")
		slot := &windowSlot{item: item, checkedMonitor: -1}
		buildMonitorSubmenusLocked(slot)
		item.Hide()
		slots = append(slots, slot)
	}

	moreItem = systray.AddMenuItem("", "")
	moreItem.Disable()
	moreItem.Hide()
}

// buildMonitorSubmenusLocked (re)builds slot's per-monitor submenu entries
// from the current 'monitors'/'monitorLabels'. Callers must hold mu
// (write lock). Used both for initial slot creation and, rarely, to
// rebuild every slot's submenus after a WM_DISPLAYCHANGE.
func buildMonitorSubmenusLocked(slot *windowSlot) {
	for _, sub := range slot.subItems {
		sub.Remove()
	}

	slot.subItems = make([]*systray.MenuItem, len(monitors))
	slot.targets = make([]slotTarget, len(monitors))
	slot.checkedMonitor = -1

	for m := range monitors {
		subItem := slot.item.AddSubMenuItemCheckbox(monitorLabels[m], "", false)
		slot.subItems[m] = subItem

		go func() {
			for range subItem.ClickedCh {
				handleSlotClick(slot, m)
			}
		}()
	}
}

func handleSlotClick(slot *windowSlot, m int) {
	mu.RLock()
	if m >= len(monitors) || m >= len(slot.targets) {
		mu.RUnlock()
		return // monitor config changed since this submenu was built; stale click
	}
	target := slot.targets[m]
	monitor := monitors[m]
	mu.RUnlock()

	if target.handle == 0 {
		return
	}

	logInfo("click: move window %v (pid %d) to monitor %d", target.handle, target.pid, m)
	moved, focused := movewindow.MoveToScreen(target.handle, target.pid, monitor)
	if !moved {
		logInfo("move aborted: window %v no longer matches the process seen at the last refresh (closed or handle reused?)", target.handle)
		return
	}
	if !focused {
		logInfo("move ok but SetForegroundWindow was refused for window %v (focus-stealing prevention)", target.handle)
	}

	rememberMonitor(target.pid, m)
}

func rememberMonitor(pid uint32, monitorIndex int) {
	if st == nil {
		return
	}
	exeName, ok := win32.ProcessExeName(pid)
	if !ok {
		return
	}
	st.Set(exeName, monitorIndex)
	go func() {
		if err := st.Save(); err != nil {
			logInfo("state.Save: %v", err)
		}
	}()
}

// refreshLoop is the safety-net poll: with the event-driven refresh (see
// handleLoopEvents) this rarely does real work, but it guarantees the menu
// eventually catches up even if a WinEventHook notification is missed, or
// if msgloop couldn't start at all.
func refreshLoop() {
	interval := time.Duration(cfg.RefreshIntervalMS) * time.Millisecond
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		refreshWindows()
	}
}

func handleLoopEvents() {
	for ev := range loop.Events {
		switch ev {
		case msgloop.EventDisplayChanged:
			logInfo("display configuration changed, re-enumerating monitors")
			handleDisplayChange()
		case msgloop.EventWindowsChanged:
			refreshWindows()
		}
	}
}

func handleDisplayChange() {
	mu.Lock()
	monitors = win32.EnumMonitors()
	monitorLabels = computeMonitorLabels(monitors)
	for _, slot := range slots {
		buildMonitorSubmenusLocked(slot)
	}
	mu.Unlock()
	logInfo("display configuration change: now %d monitors", len(monitors))
	refreshWindows()
}

func refreshWindows() {
	open := winlist.Open(winlist.Options{
		ExcludedProcessNames: cfg.ExcludedProcessNames,
		ExcludedTitles:       cfg.ExcludedTitles,
		VDesktop:             vdm,
	})

	mu.Lock()
	defer mu.Unlock()

	for i, slot := range slots {
		if i < len(open) {
			showWindowInSlotLocked(slot, open[i])
		} else {
			hideSlotLocked(slot)
		}
	}

	if len(open) > len(slots) {
		extra := len(open) - len(slots)
		moreItem.SetTitle(fmt.Sprintf("+%d more windows not shown", extra))
		moreItem.Show()
	} else {
		moreItem.Hide()
	}
}

// showWindowInSlotLocked and hideSlotLocked must be called with mu held
// (write lock). They only call into the systray/Win32 APIs that actually
// change something — SetTitle/Show/Check all trigger a native menu update
// even when called with the same value again, so skipping unchanged calls
// meaningfully cuts down on native menu churn during frequent refreshes.
func showWindowInSlotLocked(slot *windowSlot, w winlist.Window) {
	title := truncate(w.Title, 60)
	if title != slot.lastTitle {
		slot.item.SetTitle(title)
		slot.lastTitle = title
	}

	for m := range monitors {
		slot.targets[m] = slotTarget{handle: w.Handle, pid: w.PID}
	}

	lastMonitor := -1
	if st != nil && w.ExeName != "" {
		if idx, found := st.Get(w.ExeName); found && idx < len(slot.subItems) {
			lastMonitor = idx
		}
	}
	if lastMonitor != slot.checkedMonitor {
		if slot.checkedMonitor >= 0 && slot.checkedMonitor < len(slot.subItems) {
			slot.subItems[slot.checkedMonitor].Uncheck()
		}
		if lastMonitor >= 0 && lastMonitor < len(slot.subItems) {
			slot.subItems[lastMonitor].Check()
		}
		slot.checkedMonitor = lastMonitor
	}

	if !slot.visible {
		slot.item.Show()
		slot.visible = true
	}
}

func hideSlotLocked(slot *windowSlot) {
	if slot.visible {
		slot.item.Hide()
		slot.visible = false
	}
	for i := range slot.targets {
		slot.targets[i] = slotTarget{}
	}
	if slot.checkedMonitor >= 0 && slot.checkedMonitor < len(slot.subItems) {
		slot.subItems[slot.checkedMonitor].Uncheck()
		slot.checkedMonitor = -1
	}
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func registerHotkeys() {
	if loop == nil {
		return
	}
	if mods, vk, err := config.ParseHotkey(cfg.MoveNextMonitorHotkey); err != nil {
		logInfo("hotkey %q: %v", cfg.MoveNextMonitorHotkey, err)
	} else if err := loop.RegisterHotKey(hotkeyNext, mods, vk); err != nil {
		logInfo("RegisterHotKey(move-next-monitor): %v", err)
	}
	if mods, vk, err := config.ParseHotkey(cfg.MovePrevMonitorHotkey); err != nil {
		logInfo("hotkey %q: %v", cfg.MovePrevMonitorHotkey, err)
	} else if err := loop.RegisterHotKey(hotkeyPrev, mods, vk); err != nil {
		logInfo("RegisterHotKey(move-prev-monitor): %v", err)
	}
}

func handleHotkeys() {
	for id := range loop.Hotkeys {
		switch id {
		case hotkeyNext:
			moveActiveWindow(1)
		case hotkeyPrev:
			moveActiveWindow(-1)
		}
	}
}

// moveActiveWindow moves the foreground window to the next/previous
// monitor (wrapping around), triggered by a global hotkey.
func moveActiveWindow(direction int) {
	fg := w32.GetForegroundWindow()
	if fg == 0 {
		return
	}

	mu.RLock()
	n := len(monitors)
	if n == 0 {
		mu.RUnlock()
		return
	}
	cur := win32.MonitorFromWindow(fg)
	idx := 0
	for i, mon := range monitors {
		if mon.Handle == cur {
			idx = i
			break
		}
	}
	next := ((idx+direction)%n + n) % n
	target := monitors[next]
	mu.RUnlock()

	pid := win32.WindowProcessID(fg)
	logInfo("hotkey: move foreground window %v (pid %d) to monitor %d", fg, pid, next)
	moved, focused := movewindow.MoveToScreen(fg, pid, target)
	if !moved {
		logInfo("hotkey move aborted for window %v", fg)
		return
	}
	if !focused {
		logInfo("hotkey move ok but focus was refused for window %v (focus-stealing prevention)", fg)
	}
	rememberMonitor(pid, next)
}

func handleAutostartToggle() {
	for range autostartItem.ClickedCh {
		enabled, err := autostart.IsEnabled()
		if err != nil {
			logInfo("autostart.IsEnabled: %v", err)
			continue
		}

		if enabled {
			if err := autostart.Disable(); err != nil {
				logInfo("autostart.Disable: %v", err)
				continue
			}
			autostartItem.Uncheck()
			logInfo("autostart: disabled")
			continue
		}

		exe, err := os.Executable()
		if err != nil {
			logInfo("os.Executable: %v", err)
			continue
		}
		if err := autostart.Enable(exe); err != nil {
			logInfo("autostart.Enable: %v", err)
			continue
		}
		autostartItem.Check()
		logInfo("autostart: enabled")
	}
}

func handleQuit() {
	for range quitItem.ClickedCh {
		logInfo("handleQuit: received click on Quit, calling systray.Quit()")
		systray.Quit()
		return
	}
}
