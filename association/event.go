// SPDX-License-Identifier: Apache-2.0

package association

import "github.com/pravesh707/go-dicom/dimse"

// EventType identifies an association event. The single intervention event
// (EvtCEcho) is handled by a bound handler that returns a status; the
// notification events are observed and ignore the return value. This mirrors
// (a subset of) evt module — additional service events are added
// as their services are implemented.
type EventType int

const (
	// EvtCEcho is an intervention event: its handler returns a DIMSE status.
	EvtCEcho EventType = iota

	// Notification events — observed, no return value.
	EvtEstablished
	EvtReleased
	EvtAborted
	EvtDIMSERecv
)

var eventNames = map[EventType]string{
	EvtCEcho:       "EVT_C_ECHO",
	EvtEstablished: "EVT_ESTABLISHED",
	EvtReleased:    "EVT_RELEASED",
	EvtAborted:     "EVT_ABORTED",
	EvtDIMSERecv:   "EVT_DIMSE_RECV",
}

func (e EventType) String() string {
	if n, ok := eventNames[e]; ok {
		return n
	}
	return "EVT_UNKNOWN"
}

// isIntervention reports whether the event is handled by a status-returning
// service handler (as opposed to a notification observer).
func (e EventType) isIntervention() bool { return e == EvtCEcho }

// Event carries the context for a handler invocation.
type Event struct {
	Type    EventType
	Assoc   *Association
	Context AcceptedContext
	Request dimse.Message // the inbound DIMSE request (intervention events)
}

// MessageID returns the message ID of the request, when applicable.
func (e *Event) MessageID() uint16 {
	if r, ok := e.Request.(*dimse.CEchoRequest); ok {
		return r.MessageID
	}
	return 0
}

// Handler handles an intervention event and returns the DIMSE status to send
// back to the peer.
type Handler func(*Event) dimse.Status

// HandlerBinding binds an event type to a handler, tuples passed to start_server / associate.
type HandlerBinding struct {
	Event  EventType
	Handle Handler
}

// handlerTable resolves intervention handlers and fans out notification
// handlers.
type handlerTable struct {
	intervention map[EventType]Handler
	notify       map[EventType][]Handler
}

func newHandlerTable(bindings []HandlerBinding) *handlerTable {
	t := &handlerTable{
		intervention: make(map[EventType]Handler),
		notify:       make(map[EventType][]Handler),
	}
	for _, b := range bindings {
		if b.Event.isIntervention() {
			t.intervention[b.Event] = b.Handle
		} else {
			t.notify[b.Event] = append(t.notify[b.Event], b.Handle)
		}
	}
	return t
}

func (t *handlerTable) handle(ev *Event) (dimse.Status, bool) {
	if h, ok := t.intervention[ev.Type]; ok {
		return h(ev), true
	}
	return dimse.StatusSuccess, false
}

func (t *handlerTable) emit(ev *Event) {
	for _, h := range t.notify[ev.Type] {
		h(ev)
	}
}
