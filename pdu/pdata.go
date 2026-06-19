// SPDX-License-Identifier: Apache-2.0

package pdu

import (
	"encoding/binary"
	"fmt"
)

// PDV is a Presentation Data Value: a fragment of a DIMSE command or data set,
// tagged with its presentation context and two control bits.
type PDV struct {
	ContextID byte
	IsCommand bool // message control header bit 0
	IsLast    bool // message control header bit 1 (last fragment)
	Data      []byte
}

func (v PDV) messageControlHeader() byte {
	var mch byte
	if v.IsCommand {
		mch |= 0x01
	}
	if v.IsLast {
		mch |= 0x02
	}
	return mch
}

func (v PDV) encode() []byte {
	length := 2 + len(v.Data)
	out := make([]byte, 4+length)
	binary.BigEndian.PutUint32(out, uint32(length))
	out[4] = v.ContextID
	out[5] = v.messageControlHeader()
	copy(out[6:], v.Data)
	return out
}

// PDataTF is the P-DATA-TF PDU (PS3.8 §9.3.5), carrying one or more PDVs.
type PDataTF struct {
	PDVs []PDV
}

func (p *PDataTF) PDUType() byte { return TypePDataTF }

func (p *PDataTF) Encode() ([]byte, error) {
	var body []byte
	for _, v := range p.PDVs {
		body = append(body, v.encode()...)
	}
	return frame(TypePDataTF, body), nil
}

func decodePDataTF(body []byte) (*PDataTF, error) {
	p := &PDataTF{}
	pos := 0
	for pos+4 <= len(body) {
		length := int(binary.BigEndian.Uint32(body[pos:]))
		if length < 2 || pos+4+length > len(body) {
			return nil, fmt.Errorf("pdu: invalid PDV length %d", length)
		}
		ctxID := body[pos+4]
		mch := body[pos+5]
		data := make([]byte, length-2)
		copy(data, body[pos+6:pos+4+length])
		p.PDVs = append(p.PDVs, PDV{
			ContextID: ctxID,
			IsCommand: mch&0x01 != 0,
			IsLast:    mch&0x02 != 0,
			Data:      data,
		})
		pos += 4 + length
	}
	return p, nil
}
