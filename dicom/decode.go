// SPDX-License-Identifier: Apache-2.0

package dicom

import (
	"encoding/binary"
	"fmt"
)

// cursor is a forward-only reader over an encoded data set.
type cursor struct {
	data []byte
	pos  int
}

func (c *cursor) remaining() int { return len(c.data) - c.pos }

func (c *cursor) u16() (uint16, error) {
	if c.remaining() < 2 {
		return 0, fmt.Errorf("dicom: unexpected end of data reading uint16")
	}
	v := binary.LittleEndian.Uint16(c.data[c.pos:])
	c.pos += 2
	return v, nil
}

func (c *cursor) u32() (uint32, error) {
	if c.remaining() < 4 {
		return 0, fmt.Errorf("dicom: unexpected end of data reading uint32")
	}
	v := binary.LittleEndian.Uint32(c.data[c.pos:])
	c.pos += 4
	return v, nil
}

func (c *cursor) take(n int) ([]byte, error) {
	if n < 0 || c.remaining() < n {
		return nil, fmt.Errorf("dicom: unexpected end of data reading %d bytes", n)
	}
	b := c.data[c.pos : c.pos+n]
	c.pos += n
	return b, nil
}

// parseDataSet reads elements until the buffer is exhausted, or — when
// stopAtDelim is set — until an item/sequence delimitation tag is encountered
// (which is left unconsumed for the caller).
func parseDataSet(c *cursor, implicit bool, ds *DataSet, stopAtDelim bool) error {
	for c.remaining() >= 4 {
		if stopAtDelim {
			next := binary.LittleEndian.Uint16(c.data[c.pos:])
			if next == 0xFFFE {
				return nil // delimitation tag belongs to the caller
			}
		}
		e, err := parseElement(c, implicit)
		if err != nil {
			return err
		}
		ds.Set(e)
	}
	return nil
}

func parseElement(c *cursor, implicit bool) (*Element, error) {
	group, err := c.u16()
	if err != nil {
		return nil, err
	}
	elem, err := c.u16()
	if err != nil {
		return nil, err
	}
	tag := Tag{Group: group, Element: elem}

	var vr VR
	var length uint32
	if implicit {
		vr = LookupVR(tag)
		if length, err = c.u32(); err != nil {
			return nil, err
		}
	} else {
		vrBytes, err := c.take(2)
		if err != nil {
			return nil, err
		}
		vr = VR(vrBytes)
		if vr.UsesLongLength() {
			if _, err = c.take(2); err != nil { // reserved
				return nil, err
			}
			if length, err = c.u32(); err != nil {
				return nil, err
			}
		} else {
			l16, err := c.u16()
			if err != nil {
				return nil, err
			}
			length = uint32(l16)
		}
	}

	if length == undefinedLength {
		raw, err := readUndefinedLength(c, implicit)
		if err != nil {
			return nil, err
		}
		return &Element{Tag: tag, VR: vr, Raw: raw, undefined: true}, nil
	}

	val, err := c.take(int(length))
	if err != nil {
		return nil, err
	}
	// Copy so the element owns its bytes independent of the source buffer.
	raw := make([]byte, len(val))
	copy(raw, val)
	return &Element{Tag: tag, VR: vr, Raw: raw}, nil
}

// readUndefinedLength captures the raw encoded bytes of an undefined-length
// element (a sequence, or encapsulated pixel data), up to and including its
// Sequence Delimitation Item. Nested undefined-length items and sequences are
// handled by recursing into each item's sub-data-set.
func readUndefinedLength(c *cursor, implicit bool) ([]byte, error) {
	start := c.pos
	for {
		group, err := c.u16()
		if err != nil {
			return nil, err
		}
		elem, err := c.u16()
		if err != nil {
			return nil, err
		}
		itemLen, err := c.u32()
		if err != nil {
			return nil, err
		}
		if group == 0xFFFE && elem == 0xE0DD { // Sequence Delimitation Item
			break
		}
		if group == 0xFFFE && elem == 0xE000 { // Item
			if itemLen == undefinedLength {
				sub := NewDataSet()
				if err := parseDataSet(c, implicit, sub, true); err != nil {
					return nil, err
				}
				// consume the Item Delimitation Item (FFFE,E00D) + length
				if _, err := c.take(8); err != nil {
					return nil, err
				}
			} else {
				if _, err := c.take(int(itemLen)); err != nil {
					return nil, err
				}
			}
			continue
		}
		return nil, fmt.Errorf("dicom: unexpected tag (%04X,%04X) in undefined-length value", group, elem)
	}
	raw := make([]byte, c.pos-start)
	copy(raw, c.data[start:c.pos])
	return raw, nil
}
