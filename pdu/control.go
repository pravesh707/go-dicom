// SPDX-License-Identifier: Apache-2.0

package pdu

import "fmt"

// ReleaseRQ is the A-RELEASE-RQ PDU (PS3.8 §9.3.6).
type ReleaseRQ struct{}

func (p *ReleaseRQ) PDUType() byte           { return TypeReleaseRQ }
func (p *ReleaseRQ) Encode() ([]byte, error) { return frame(TypeReleaseRQ, make([]byte, 4)), nil }

func decodeReleaseRQ(_ []byte) (*ReleaseRQ, error) { return &ReleaseRQ{}, nil }

// ReleaseRP is the A-RELEASE-RP PDU (PS3.8 §9.3.7).
type ReleaseRP struct{}

func (p *ReleaseRP) PDUType() byte           { return TypeReleaseRP }
func (p *ReleaseRP) Encode() ([]byte, error) { return frame(TypeReleaseRP, make([]byte, 4)), nil }

func decodeReleaseRP(_ []byte) (*ReleaseRP, error) { return &ReleaseRP{}, nil }

// A-ABORT source / reason (PS3.8 Table 9-26).
const (
	AbortSourceServiceUser     byte = 0x00
	AbortSourceServiceProvider byte = 0x02

	AbortReasonNotSpecified         byte = 0x00
	AbortReasonUnrecognizedPDU      byte = 0x01
	AbortReasonUnexpectedPDU        byte = 0x02
	AbortReasonUnrecognizedPDUParam byte = 0x04
	AbortReasonUnexpectedPDUParam   byte = 0x05
	AbortReasonInvalidPDUParam      byte = 0x06
)

// Abort is the A-ABORT PDU (PS3.8 §9.3.8).
type Abort struct {
	Source byte
	Reason byte
}

func (p *Abort) PDUType() byte { return TypeAbort }

func (p *Abort) Encode() ([]byte, error) {
	return frame(TypeAbort, []byte{0x00, 0x00, p.Source, p.Reason}), nil
}

func decodeAbort(body []byte) (*Abort, error) {
	if len(body) < 4 {
		return nil, fmt.Errorf("pdu: A-ABORT too short")
	}
	return &Abort{Source: body[2], Reason: body[3]}, nil
}
