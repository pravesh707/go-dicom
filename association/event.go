// SPDX-License-Identifier: Apache-2.0

package association

import (
	"fmt"

	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
)

// EventType identifies an association event. Intervention events (EvtCEcho …
// EvtNDelete) are each handled by one bound handler that returns a DIMSE
// status; notification events are observed and ignore the return value. This
// mirrors pynetdicom's evt module.
type EventType int

const (
	// Intervention events — handled by a status-returning service handler.
	EvtCEcho EventType = iota
	EvtCStore
	EvtCFind
	EvtCGet
	EvtCMove
	EvtNEventReport
	EvtNGet
	EvtNSet
	EvtNAction
	EvtNCreate
	EvtNDelete

	// Notification events — observed, no return value.
	EvtEstablished
	EvtReleased
	EvtAborted
	EvtRequested
	EvtAccepted
	EvtRejected
	EvtDIMSERecv
	EvtDIMSESent
)

// lastIntervention is the highest-valued intervention event.
const lastIntervention = EvtNDelete

var eventNames = map[EventType]string{
	EvtCEcho:        "EVT_C_ECHO",
	EvtCStore:       "EVT_C_STORE",
	EvtCFind:        "EVT_C_FIND",
	EvtCGet:         "EVT_C_GET",
	EvtCMove:        "EVT_C_MOVE",
	EvtNEventReport: "EVT_N_EVENT_REPORT",
	EvtNGet:         "EVT_N_GET",
	EvtNSet:         "EVT_N_SET",
	EvtNAction:      "EVT_N_ACTION",
	EvtNCreate:      "EVT_N_CREATE",
	EvtNDelete:      "EVT_N_DELETE",
	EvtEstablished:  "EVT_ESTABLISHED",
	EvtReleased:     "EVT_RELEASED",
	EvtAborted:      "EVT_ABORTED",
	EvtRequested:    "EVT_REQUESTED",
	EvtAccepted:     "EVT_ACCEPTED",
	EvtRejected:     "EVT_REJECTED",
	EvtDIMSERecv:    "EVT_DIMSE_RECV",
	EvtDIMSESent:    "EVT_DIMSE_SENT",
}

func (e EventType) String() string {
	if n, ok := eventNames[e]; ok {
		return n
	}
	return "EVT_UNKNOWN"
}

// isIntervention reports whether the event is handled by a status-returning
// service handler (as opposed to a notification observer).
func (e EventType) isIntervention() bool { return e <= lastIntervention }

// Event carries the context for a handler invocation.
type Event struct {
	Type    EventType
	Assoc   *Association
	Context AcceptedContext
	Request dimse.Message  // the inbound DIMSE request (intervention events)
	DataSet *dicom.DataSet // the inbound data set / identifier, if any

	// yield streams an intermediate (pending) response for query/retrieve
	// services (C-FIND/C-GET/C-MOVE); nil for other events.
	yield func(*dicom.DataSet) error

	// response holds the data set a DIMSE-N handler returns to the peer
	// (e.g. the attribute list for N-GET); set via SetResponse.
	response *dicom.DataSet
}

// SetResponse attaches a data set to be returned with the DIMSE-N response
// (e.g. the requested attributes for N-GET, or the created instance for
// N-CREATE).
func (e *Event) SetResponse(ds *dicom.DataSet) { e.response = ds }

// MessageID returns the message ID of the inbound request, when applicable.
func (e *Event) MessageID() uint16 {
	if r, ok := e.Request.(interface{ GetMessageID() uint16 }); ok {
		return r.GetMessageID()
	}
	return 0
}

// Yield streams one intermediate result from a C-FIND/C-GET/C-MOVE handler: for
// C-FIND it sends a Pending response carrying the match identifier; for
// C-GET/C-MOVE it stores the instance to the destination. It returns an error
// if called outside a query/retrieve handler or if the send fails.
func (e *Event) Yield(ds *dicom.DataSet) error {
	if e.yield == nil {
		return fmt.Errorf("godicom: Yield called outside a C-FIND/C-GET/C-MOVE handler")
	}
	return e.yield(ds)
}

// Handler handles an intervention event and returns the DIMSE status to send
// back to the peer.
type Handler func(*Event) dimse.Status

// HandlerBinding binds an event type to a handler, mirroring pynetdicom's
// (event, handler) tuples passed to start_server / associate.
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
