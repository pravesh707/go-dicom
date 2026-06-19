// SPDX-License-Identifier: Apache-2.0

package pdu

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// protocolVersion is the only defined DICOM UL protocol version.
const protocolVersion = 0x0001

func encodeAETitle(title string) []byte {
	b := []byte("                ") // 16 spaces
	copy(b, []byte(title))
	if len(title) > 16 {
		copy(b, []byte(title)[:16])
	}
	return b
}

func decodeAETitle(b []byte) string {
	return strings.TrimSpace(string(b))
}

// AssociateRQ is the A-ASSOCIATE-RQ PDU (PS3.8 §9.3.2).
type AssociateRQ struct {
	CalledAETitle        string
	CallingAETitle       string
	ApplicationContext   string
	PresentationContexts []PresentationContextRQ
	UserInformation      UserInformation
}

func (p *AssociateRQ) PDUType() byte { return TypeAssociateRQ }

func (p *AssociateRQ) Encode() ([]byte, error) {
	var body []byte
	var fixed [2]byte
	binary.BigEndian.PutUint16(fixed[:], protocolVersion)
	body = append(body, fixed[0], fixed[1], 0x00, 0x00)
	body = append(body, encodeAETitle(p.CalledAETitle)...)
	body = append(body, encodeAETitle(p.CallingAETitle)...)
	body = append(body, make([]byte, 32)...) // reserved
	body = append(body, encodeItem(itemApplicationContext, []byte(p.ApplicationContext))...)
	for _, pc := range p.PresentationContexts {
		body = append(body, pc.encode()...)
	}
	body = append(body, p.UserInformation.encode()...)
	return frame(TypeAssociateRQ, body), nil
}

func decodeAssociateRQ(body []byte) (*AssociateRQ, error) {
	if len(body) < 68 {
		return nil, fmt.Errorf("pdu: A-ASSOCIATE-RQ too short (%d bytes)", len(body))
	}
	p := &AssociateRQ{
		CalledAETitle:  decodeAETitle(body[4:20]),
		CallingAETitle: decodeAETitle(body[20:36]),
	}
	r := itemReader{data: body[68:]}
	for r.more() {
		t, b, err := r.next()
		if err != nil {
			return nil, err
		}
		switch t {
		case itemApplicationContext:
			p.ApplicationContext = string(b)
		case itemPresentationCtxRQ:
			pc, err := decodePresentationContextRQ(b)
			if err != nil {
				return nil, err
			}
			p.PresentationContexts = append(p.PresentationContexts, pc)
		case itemUserInformation:
			ui, err := decodeUserInformation(b)
			if err != nil {
				return nil, err
			}
			p.UserInformation = ui
		}
	}
	return p, nil
}

// AssociateAC is the A-ASSOCIATE-AC PDU (PS3.8 §9.3.3).
type AssociateAC struct {
	CalledAETitle        string // reserved fields, echoed from the request
	CallingAETitle       string
	ApplicationContext   string
	PresentationContexts []PresentationContextAC
	UserInformation      UserInformation
}

func (p *AssociateAC) PDUType() byte { return TypeAssociateAC }

func (p *AssociateAC) Encode() ([]byte, error) {
	var body []byte
	var fixed [2]byte
	binary.BigEndian.PutUint16(fixed[:], protocolVersion)
	body = append(body, fixed[0], fixed[1], 0x00, 0x00)
	body = append(body, encodeAETitle(p.CalledAETitle)...)
	body = append(body, encodeAETitle(p.CallingAETitle)...)
	body = append(body, make([]byte, 32)...) // reserved
	body = append(body, encodeItem(itemApplicationContext, []byte(p.ApplicationContext))...)
	for _, pc := range p.PresentationContexts {
		body = append(body, pc.encode()...)
	}
	body = append(body, p.UserInformation.encode()...)
	return frame(TypeAssociateAC, body), nil
}

func decodeAssociateAC(body []byte) (*AssociateAC, error) {
	if len(body) < 68 {
		return nil, fmt.Errorf("pdu: A-ASSOCIATE-AC too short (%d bytes)", len(body))
	}
	p := &AssociateAC{
		CalledAETitle:  decodeAETitle(body[4:20]),
		CallingAETitle: decodeAETitle(body[20:36]),
	}
	r := itemReader{data: body[68:]}
	for r.more() {
		t, b, err := r.next()
		if err != nil {
			return nil, err
		}
		switch t {
		case itemApplicationContext:
			p.ApplicationContext = string(b)
		case itemPresentationCtxAC:
			pc, err := decodePresentationContextAC(b)
			if err != nil {
				return nil, err
			}
			p.PresentationContexts = append(p.PresentationContexts, pc)
		case itemUserInformation:
			ui, err := decodeUserInformation(b)
			if err != nil {
				return nil, err
			}
			p.UserInformation = ui
		}
	}
	return p, nil
}

// A-ASSOCIATE-RJ result / source / reason (PS3.8 Table 9-21).
const (
	RJResultRejectedPermanent byte = 0x01
	RJResultRejectedTransient byte = 0x02

	RJSourceServiceUser         byte = 0x01
	RJSourceServiceProviderACSE byte = 0x02
	RJSourceServiceProviderPres byte = 0x03

	RJReasonNoReasonGiven          byte = 0x01
	RJReasonAppContextNotSupported byte = 0x02
	RJReasonCallingAENotRecognized byte = 0x03
	RJReasonCalledAENotRecognized  byte = 0x07
)

// AssociateRJ is the A-ASSOCIATE-RJ PDU (PS3.8 §9.3.4).
type AssociateRJ struct {
	Result byte
	Source byte
	Reason byte
}

func (p *AssociateRJ) PDUType() byte { return TypeAssociateRJ }

func (p *AssociateRJ) Encode() ([]byte, error) {
	body := []byte{0x00, p.Result, p.Source, p.Reason}
	return frame(TypeAssociateRJ, body), nil
}

func decodeAssociateRJ(body []byte) (*AssociateRJ, error) {
	if len(body) < 4 {
		return nil, fmt.Errorf("pdu: A-ASSOCIATE-RJ too short")
	}
	return &AssociateRJ{Result: body[1], Source: body[2], Reason: body[3]}, nil
}
