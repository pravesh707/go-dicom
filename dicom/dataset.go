// SPDX-License-Identifier: Apache-2.0

package dicom

import "sort"

// DataSet is an ordered collection of DICOM data elements keyed by tag. On
// encoding, elements are emitted in ascending tag order as required by PS3.5.
type DataSet struct {
	elements map[Tag]*Element
}

// NewDataSet returns an empty data set.
func NewDataSet() *DataSet {
	return &DataSet{elements: make(map[Tag]*Element)}
}

// Set inserts or replaces an element.
func (ds *DataSet) Set(e *Element) {
	if ds.elements == nil {
		ds.elements = make(map[Tag]*Element)
	}
	ds.elements[e.Tag] = e
}

// Get returns the element for a tag.
func (ds *DataSet) Get(tag Tag) (*Element, bool) {
	e, ok := ds.elements[tag]
	return e, ok
}

// Has reports whether the tag is present.
func (ds *DataSet) Has(tag Tag) bool {
	_, ok := ds.elements[tag]
	return ok
}

// Remove deletes the element for a tag, if present.
func (ds *DataSet) Remove(tag Tag) { delete(ds.elements, tag) }

// Len returns the number of elements.
func (ds *DataSet) Len() int { return len(ds.elements) }

// Tags returns the element tags in ascending (group, element) order.
func (ds *DataSet) Tags() []Tag {
	tags := make([]Tag, 0, len(ds.elements))
	for t := range ds.elements {
		tags = append(tags, t)
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Less(tags[j]) })
	return tags
}

// Elements returns the elements in ascending tag order.
func (ds *DataSet) Elements() []*Element {
	tags := ds.Tags()
	out := make([]*Element, len(tags))
	for i, t := range tags {
		out[i] = ds.elements[t]
	}
	return out
}

// ---- Convenience typed accessors ----

// GetUint16 returns the first uint16 value of a tag.
func (ds *DataSet) GetUint16(tag Tag) (uint16, bool) {
	if e, ok := ds.elements[tag]; ok {
		return e.Uint16(), true
	}
	return 0, false
}

// GetUint32 returns the first uint32 value of a tag.
func (ds *DataSet) GetUint32(tag Tag) (uint32, bool) {
	if e, ok := ds.elements[tag]; ok {
		return e.Uint32(), true
	}
	return 0, false
}

// GetString returns the trimmed text value of a tag.
func (ds *DataSet) GetString(tag Tag) (string, bool) {
	if e, ok := ds.elements[tag]; ok {
		return e.String(), true
	}
	return "", false
}
