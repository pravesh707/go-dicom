// SPDX-License-Identifier: Apache-2.0

package dicom

// Commonly used data-element tags (PS3.6). This is a convenience subset; use
// Tag{group, element} or RegisterDictEntry for anything not listed here.
var (
	TagSpecificCharacterSet = Tag{0x0008, 0x0005}
	TagSOPClassUID          = Tag{0x0008, 0x0016}
	TagSOPInstanceUID       = Tag{0x0008, 0x0018}
	TagStudyDate            = Tag{0x0008, 0x0020}
	TagStudyTime            = Tag{0x0008, 0x0030}
	TagAccessionNumber      = Tag{0x0008, 0x0050}
	TagQueryRetrieveLevel   = Tag{0x0008, 0x0052}
	TagModalitiesInStudy    = Tag{0x0008, 0x0061}
	TagModality             = Tag{0x0008, 0x0060}
	TagReferringPhysician   = Tag{0x0008, 0x0090}
	TagPatientName          = Tag{0x0010, 0x0010}
	TagPatientID            = Tag{0x0010, 0x0020}
	TagPatientBirthDate     = Tag{0x0010, 0x0030}
	TagPatientSex           = Tag{0x0010, 0x0040}
	TagStudyInstanceUID     = Tag{0x0020, 0x000D}
	TagSeriesInstanceUID    = Tag{0x0020, 0x000E}
	TagStudyID              = Tag{0x0020, 0x0010}
	TagSeriesNumber         = Tag{0x0020, 0x0011}
	TagInstanceNumber       = Tag{0x0020, 0x0013}
	TagRows                 = Tag{0x0028, 0x0010}
	TagColumns              = Tag{0x0028, 0x0011}
)

// NewTag is a convenience constructor for a Tag from group and element numbers.
func NewTag(group, element uint16) Tag { return Tag{Group: group, Element: element} }
