// SPDX-License-Identifier: Apache-2.0

package pdu

import (
	"encoding/binary"
	"fmt"
)

// Item / sub-item type codes (PS3.8 Table 9-16 and following).
const (
	itemApplicationContext  byte = 0x10
	itemPresentationCtxRQ   byte = 0x20
	itemPresentationCtxAC   byte = 0x21
	itemAbstractSyntax      byte = 0x30
	itemTransferSyntax      byte = 0x40
	itemUserInformation     byte = 0x50
	itemMaximumLength       byte = 0x51
	itemImplClassUID        byte = 0x52
	itemAsyncOpsWindow      byte = 0x53
	itemRoleSelection       byte = 0x54
	itemImplVersionName     byte = 0x55
	itemExtendedNegotiation byte = 0x56
	itemUserIdentityRQ      byte = 0x58
	itemUserIdentityAC      byte = 0x59
)

// encodeItem builds an item/sub-item: type(1) reserved(1) length(2 BE) data.
func encodeItem(itemType byte, data []byte) []byte {
	out := make([]byte, 4+len(data))
	out[0] = itemType
	binary.BigEndian.PutUint16(out[2:], uint16(len(data)))
	copy(out[4:], data)
	return out
}

// itemReader iterates the items packed into a byte slice.
type itemReader struct {
	data []byte
	pos  int
}

func (r *itemReader) more() bool { return r.pos+4 <= len(r.data) }

func (r *itemReader) next() (itemType byte, body []byte, err error) {
	if r.pos+4 > len(r.data) {
		return 0, nil, fmt.Errorf("pdu: truncated item header")
	}
	itemType = r.data[r.pos]
	length := int(binary.BigEndian.Uint16(r.data[r.pos+2:]))
	start := r.pos + 4
	if start+length > len(r.data) {
		return 0, nil, fmt.Errorf("pdu: item %#x length %d exceeds buffer", itemType, length)
	}
	body = r.data[start : start+length]
	r.pos = start + length
	return itemType, body, nil
}

// ---- Presentation context (request side) ----

// PresentationContextRQ is one proposed presentation context inside an
// A-ASSOCIATE-RQ: an abstract syntax with one or more candidate transfer
// syntaxes, identified by an odd presentation context ID.
type PresentationContextRQ struct {
	ID               byte
	AbstractSyntax   string
	TransferSyntaxes []string
}

func (pc PresentationContextRQ) encode() []byte {
	var body []byte
	body = append(body, pc.ID, 0x00, 0x00, 0x00)
	body = append(body, encodeItem(itemAbstractSyntax, []byte(pc.AbstractSyntax))...)
	for _, ts := range pc.TransferSyntaxes {
		body = append(body, encodeItem(itemTransferSyntax, []byte(ts))...)
	}
	return encodeItem(itemPresentationCtxRQ, body)
}

func decodePresentationContextRQ(body []byte) (PresentationContextRQ, error) {
	if len(body) < 4 {
		return PresentationContextRQ{}, fmt.Errorf("pdu: short presentation context RQ")
	}
	pc := PresentationContextRQ{ID: body[0]}
	r := itemReader{data: body[4:]}
	for r.more() {
		t, b, err := r.next()
		if err != nil {
			return pc, err
		}
		switch t {
		case itemAbstractSyntax:
			pc.AbstractSyntax = string(b)
		case itemTransferSyntax:
			pc.TransferSyntaxes = append(pc.TransferSyntaxes, string(b))
		}
	}
	return pc, nil
}

// ---- Presentation context (accept side) ----

// Presentation context negotiation results (PS3.8 Table 9-18).
const (
	PCAccepted             byte = 0x00
	PCUserRejection        byte = 0x01
	PCNoReason             byte = 0x02
	PCAbstractSyntaxNotSup byte = 0x03
	PCTransferSyntaxNotSup byte = 0x04
)

// PresentationContextAC is the accept-side result for one presentation context.
type PresentationContextAC struct {
	ID             byte
	Result         byte
	TransferSyntax string
}

func (pc PresentationContextAC) encode() []byte {
	var body []byte
	body = append(body, pc.ID, 0x00, pc.Result, 0x00)
	body = append(body, encodeItem(itemTransferSyntax, []byte(pc.TransferSyntax))...)
	return encodeItem(itemPresentationCtxAC, body)
}

func decodePresentationContextAC(body []byte) (PresentationContextAC, error) {
	if len(body) < 4 {
		return PresentationContextAC{}, fmt.Errorf("pdu: short presentation context AC")
	}
	pc := PresentationContextAC{ID: body[0], Result: body[2]}
	r := itemReader{data: body[4:]}
	for r.more() {
		t, b, err := r.next()
		if err != nil {
			return pc, err
		}
		if t == itemTransferSyntax {
			pc.TransferSyntax = string(b)
		}
	}
	return pc, nil
}

// ---- User information ----

// RoleSelection is an SCP/SCU Role Selection sub-item (PS3.7 §D.3.3.4).
type RoleSelection struct {
	SOPClassUID string
	SCURole     bool
	SCPRole     bool
}

func (rs RoleSelection) encode() []byte {
	uid := []byte(rs.SOPClassUID)
	body := make([]byte, 2+len(uid)+2)
	binary.BigEndian.PutUint16(body, uint16(len(uid)))
	copy(body[2:], uid)
	if rs.SCURole {
		body[2+len(uid)] = 1
	}
	if rs.SCPRole {
		body[2+len(uid)+1] = 1
	}
	return encodeItem(itemRoleSelection, body)
}

func decodeRoleSelection(body []byte) (RoleSelection, error) {
	if len(body) < 4 {
		return RoleSelection{}, fmt.Errorf("pdu: short role selection")
	}
	n := int(binary.BigEndian.Uint16(body))
	if 2+n+2 > len(body) {
		return RoleSelection{}, fmt.Errorf("pdu: role selection UID length overflow")
	}
	return RoleSelection{
		SOPClassUID: string(body[2 : 2+n]),
		SCURole:     body[2+n] != 0,
		SCPRole:     body[2+n+1] != 0,
	}, nil
}

// UserInformation is the User Information item (0x50) carried in both the
// request and accept association PDUs.
type UserInformation struct {
	MaximumLength             uint32 // 0 means "no maximum"
	ImplementationClassUID    string
	ImplementationVersionName string
	MaxOpsInvoked             uint16 // 0 => async window absent
	MaxOpsPerformed           uint16
	hasAsyncOps               bool
	RoleSelection             []RoleSelection
	// Raw unparsed sub-items (extended negotiation, user identity, ...),
	// preserved so they survive a decode/encode round trip.
	Extra [][2]any // [type byte (as int), body []byte]
}

// SetAsyncOps records an Asynchronous Operations Window sub-item.
func (ui *UserInformation) SetAsyncOps(invoked, performed uint16) {
	ui.MaxOpsInvoked, ui.MaxOpsPerformed, ui.hasAsyncOps = invoked, performed, true
}

func (ui UserInformation) encode() []byte {
	var body []byte

	max := make([]byte, 4)
	binary.BigEndian.PutUint32(max, ui.MaximumLength)
	body = append(body, encodeItem(itemMaximumLength, max)...)

	if ui.ImplementationClassUID != "" {
		body = append(body, encodeItem(itemImplClassUID, []byte(ui.ImplementationClassUID))...)
	}
	if ui.hasAsyncOps {
		aw := make([]byte, 4)
		binary.BigEndian.PutUint16(aw, ui.MaxOpsInvoked)
		binary.BigEndian.PutUint16(aw[2:], ui.MaxOpsPerformed)
		body = append(body, encodeItem(itemAsyncOpsWindow, aw)...)
	}
	for _, rs := range ui.RoleSelection {
		body = append(body, rs.encode()...)
	}
	if ui.ImplementationVersionName != "" {
		body = append(body, encodeItem(itemImplVersionName, []byte(ui.ImplementationVersionName))...)
	}
	for _, ex := range ui.Extra {
		body = append(body, encodeItem(byte(ex[0].(int)), ex[1].([]byte))...)
	}
	return encodeItem(itemUserInformation, body)
}

func decodeUserInformation(body []byte) (UserInformation, error) {
	var ui UserInformation
	r := itemReader{data: body}
	for r.more() {
		t, b, err := r.next()
		if err != nil {
			return ui, err
		}
		switch t {
		case itemMaximumLength:
			if len(b) >= 4 {
				ui.MaximumLength = binary.BigEndian.Uint32(b)
			}
		case itemImplClassUID:
			ui.ImplementationClassUID = string(b)
		case itemImplVersionName:
			ui.ImplementationVersionName = string(b)
		case itemAsyncOpsWindow:
			if len(b) >= 4 {
				ui.SetAsyncOps(binary.BigEndian.Uint16(b), binary.BigEndian.Uint16(b[2:]))
			}
		case itemRoleSelection:
			rs, err := decodeRoleSelection(b)
			if err != nil {
				return ui, err
			}
			ui.RoleSelection = append(ui.RoleSelection, rs)
		default:
			cp := make([]byte, len(b))
			copy(cp, b)
			ui.Extra = append(ui.Extra, [2]any{int(t), cp})
		}
	}
	return ui, nil
}
