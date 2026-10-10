// Package desktopnotify shows "now playing" notifications through the
// freedesktop notification service on the D-Bus session bus (mako, dunst,
// swaync, GNOME, KDE...). Each notification replaces the previous one, and
// when the server supports actions it offers Next and Like buttons.
package desktopnotify

import (
	"fmt"
	"slices"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	busName = "org.freedesktop.Notifications"
	objPath = "/org/freedesktop/Notifications"
	iface   = "org.freedesktop.Notifications"
	// timeoutMS is how long a notification stays up.
	timeoutMS = 6000
)

// Action keys sent to the action callback.
const (
	ActionNext = "next"
	ActionLike = "like"
)

// Notifier sends notifications. It is safe for concurrent use.
type Notifier struct {
	conn     *dbus.Conn
	obj      dbus.BusObject
	actions  bool
	labels   [2]string // Next and Like button labels
	onAction func(action string)

	mu     sync.Mutex
	lastID uint32
}

// New connects to the session bus. onAction gets the key of a pressed
// button; next and like are the button labels.
func New(next, like string, onAction func(action string)) (*Notifier, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("desktop notifications: %w", err)
	}
	n := &Notifier{conn: conn, obj: conn.Object(busName, objPath), labels: [2]string{next, like}, onAction: onAction}
	var caps []string
	if err := n.obj.Call(iface+".GetCapabilities", 0).Store(&caps); err != nil {
		conn.Close()
		return nil, fmt.Errorf("desktop notifications: no notification server: %w", err)
	}
	n.actions = slices.Contains(caps, "actions")
	if n.actions && onAction != nil {
		if err := conn.AddMatchSignal(dbus.WithMatchInterface(iface), dbus.WithMatchMember("ActionInvoked")); err == nil {
			ch := make(chan *dbus.Signal, 8)
			conn.Signal(ch)
			go n.watch(ch)
		}
	}
	return n, nil
}

func (n *Notifier) watch(ch chan *dbus.Signal) {
	for sig := range ch {
		if sig.Name != iface+".ActionInvoked" || len(sig.Body) < 2 {
			continue
		}
		id, _ := sig.Body[0].(uint32)
		key, _ := sig.Body[1].(string)
		n.mu.Lock()
		ours := id == n.lastID
		n.mu.Unlock()
		if ours && (key == ActionNext || key == ActionLike) {
			n.onAction(key)
		}
	}
}

// Show shows or replaces the now-playing notification. image is a path to
// a cover picture, or "".
func (n *Notifier) Show(summary, body, image string) error {
	if n == nil {
		return nil
	}
	var actions []string
	if n.actions {
		actions = []string{ActionNext, n.labels[0], ActionLike, n.labels[1]}
	}
	hints := map[string]dbus.Variant{
		"desktop-entry": dbus.MakeVariant("cliamp-x"),
		"category":      dbus.MakeVariant("x-cliamp.nowplaying"),
		"transient":     dbus.MakeVariant(true),
	}
	if image != "" {
		hints["image-path"] = dbus.MakeVariant(image)
	}
	n.mu.Lock()
	replaces := n.lastID
	n.mu.Unlock()
	var id uint32
	err := n.obj.Call(iface+".Notify", 0, "cliamp-x", replaces, "audio-x-generic", summary, body, actions, hints, int32(timeoutMS)).Store(&id)
	if err != nil {
		return fmt.Errorf("desktop notifications: %w", err)
	}
	n.mu.Lock()
	n.lastID = id
	n.mu.Unlock()
	return nil
}

// Close disconnects from the bus.
func (n *Notifier) Close() {
	if n != nil && n.conn != nil {
		n.conn.Close()
	}
}
