// SPDX-License-Identifier: Apache-2.0

// Package pdu implements the DICOM Upper Layer Protocol Data Units (PS3.8 §9):
// A-ASSOCIATE-RQ/AC/RJ, P-DATA-TF, A-RELEASE-RQ/RP and A-ABORT, together with
// the items and sub-items carried inside the association PDUs.
//
// Every PDU has a six-byte header: a one-byte type, one reserved byte, and a
// four-byte big-endian length of the remaining bytes.
package pdu

import (
	"encoding/binary"
	"fmt"
	"io"
)

// PDU type codes (PS3.8 Table 9-11).
const (
	TypeAssociateRQ byte = 0x01
	TypeAssociateAC byte = 0x02
	TypeAssociateRJ byte = 0x03
	TypePDataTF     byte = 0x04
	TypeReleaseRQ   byte = 0x05
	TypeReleaseRP   byte = 0x06
	TypeAbort       byte = 0x07
)

// PDU is implemented by every Upper Layer PDU.
type PDU interface {
	// PDUType returns the one-byte PDU type code.
	PDUType() byte
	// Encode returns the complete PDU including its six-byte header.
	Encode() ([]byte, error)
}

// frame prepends the standard six-byte PDU header to a body.
func frame(pduType byte, body []byte) []byte {
	out := make([]byte, 6+len(body))
	out[0] = pduType
	out[1] = 0x00
	binary.BigEndian.PutUint32(out[2:], uint32(len(body)))
	copy(out[6:], body)
	return out
}

// WritePDU encodes a PDU and writes it to w.
func WritePDU(w io.Writer, p PDU) error {
	b, err := p.Encode()
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// ReadPDU reads one PDU from r, dispatching on the type code. It blocks until a
// full PDU has been read or an error (including io.EOF) occurs.
func ReadPDU(r io.Reader) (PDU, error) {
	var header [6]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	pduType := header[0]
	length := binary.BigEndian.Uint32(header[2:])
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("pdu: reading body of type %#x: %w", pduType, err)
	}
	return Decode(pduType, body)
}

// Parser decodes a PDU body (the bytes after the six-byte header) into a PDU.
type Parser func(body []byte) (PDU, error)

// decoderRegistry maps a PDU type code to its parser. Registering a new PDU
// type does not require editing Decode (Open/Closed).
var decoderRegistry = map[byte]Parser{}

// RegisterPDU registers a parser for a PDU type code. Intended for package
// init; not safe for concurrent use with Decode.
func RegisterPDU(pduType byte, p Parser) { decoderRegistry[pduType] = p }

func init() {
	RegisterPDU(TypeAssociateRQ, func(b []byte) (PDU, error) { return decodeAssociateRQ(b) })
	RegisterPDU(TypeAssociateAC, func(b []byte) (PDU, error) { return decodeAssociateAC(b) })
	RegisterPDU(TypeAssociateRJ, func(b []byte) (PDU, error) { return decodeAssociateRJ(b) })
	RegisterPDU(TypePDataTF, func(b []byte) (PDU, error) { return decodePDataTF(b) })
	RegisterPDU(TypeReleaseRQ, func(b []byte) (PDU, error) { return decodeReleaseRQ(b) })
	RegisterPDU(TypeReleaseRP, func(b []byte) (PDU, error) { return decodeReleaseRP(b) })
	RegisterPDU(TypeAbort, func(b []byte) (PDU, error) { return decodeAbort(b) })
}

// Decode parses a PDU body for the given type code using the registry.
func Decode(pduType byte, body []byte) (PDU, error) {
	if parse, ok := decoderRegistry[pduType]; ok {
		return parse(body)
	}
	return nil, fmt.Errorf("pdu: unknown PDU type %#x", pduType)
}
