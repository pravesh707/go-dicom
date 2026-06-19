// SPDX-License-Identifier: Apache-2.0

package dicom

// DictEntry describes one data dictionary entry.
type DictEntry struct {
	VR      VR
	VM      string // value multiplicity, e.g. "1", "1-n"
	Keyword string
}

// dictionary maps a tag to its dictionary entry. It contains the full command
// group (0000) plus a practical subset of common data-set attributes. It is
// intentionally extensible: callers may register additional entries with
// RegisterDictEntry, and unknown tags resolve to VR UN (see LookupVR).
var dictionary = map[Tag]DictEntry{
	// ---- Command group (0000) — PS3.7 ----
	TagCommandGroupLength:        {VRUL, "1", "CommandGroupLength"},
	TagAffectedSOPClassUID:       {VRUI, "1", "AffectedSOPClassUID"},
	TagRequestedSOPClassUID:      {VRUI, "1", "RequestedSOPClassUID"},
	TagCommandField:              {VRUS, "1", "CommandField"},
	TagMessageID:                 {VRUS, "1", "MessageID"},
	TagMessageIDBeingRespondedTo: {VRUS, "1", "MessageIDBeingRespondedTo"},
	TagMoveDestination:           {VRAE, "1", "MoveDestination"},
	TagPriority:                  {VRUS, "1", "Priority"},
	TagCommandDataSetType:        {VRUS, "1", "CommandDataSetType"},
	TagStatus:                    {VRUS, "1", "Status"},
	TagOffendingElement:          {VRAT, "1-n", "OffendingElement"},
	TagErrorComment:              {VRLO, "1", "ErrorComment"},
	TagErrorID:                   {VRUS, "1", "ErrorID"},
	TagAffectedSOPInstanceUID:    {VRUI, "1", "AffectedSOPInstanceUID"},
	TagRequestedSOPInstanceUID:   {VRUI, "1", "RequestedSOPInstanceUID"},
	TagEventTypeID:               {VRUS, "1", "EventTypeID"},
	TagAttributeIdentifierList:   {VRAT, "1-n", "AttributeIdentifierList"},
	TagActionTypeID:              {VRUS, "1", "ActionTypeID"},
	TagNumberOfRemainingSubops:   {VRUS, "1", "NumberOfRemainingSuboperations"},
	TagNumberOfCompletedSubops:   {VRUS, "1", "NumberOfCompletedSuboperations"},
	TagNumberOfFailedSubops:      {VRUS, "1", "NumberOfFailedSuboperations"},
	TagNumberOfWarningSubops:     {VRUS, "1", "NumberOfWarningSuboperations"},
	TagMoveOriginatorAETitle:     {VRAE, "1", "MoveOriginatorApplicationEntityTitle"},
	TagMoveOriginatorMessageID:   {VRUS, "1", "MoveOriginatorMessageID"},

	// ---- Common data-set attributes (subset; extend as needed) ----
	{0x0008, 0x0005}: {VRCS, "1-n", "SpecificCharacterSet"},
	{0x0008, 0x0016}: {VRUI, "1", "SOPClassUID"},
	{0x0008, 0x0018}: {VRUI, "1", "SOPInstanceUID"},
	{0x0008, 0x0020}: {VRDA, "1", "StudyDate"},
	{0x0008, 0x0030}: {VRTM, "1", "StudyTime"},
	{0x0008, 0x0050}: {VRSH, "1", "AccessionNumber"},
	{0x0008, 0x0052}: {VRCS, "1", "QueryRetrieveLevel"},
	{0x0008, 0x0054}: {VRAE, "1-n", "RetrieveAETitle"},
	{0x0008, 0x0060}: {VRCS, "1", "Modality"},
	{0x0008, 0x0090}: {VRPN, "1", "ReferringPhysicianName"},
	{0x0010, 0x0010}: {VRPN, "1", "PatientName"},
	{0x0010, 0x0020}: {VRLO, "1", "PatientID"},
	{0x0010, 0x0030}: {VRDA, "1", "PatientBirthDate"},
	{0x0010, 0x0040}: {VRCS, "1", "PatientSex"},
	{0x0020, 0x000D}: {VRUI, "1", "StudyInstanceUID"},
	{0x0020, 0x000E}: {VRUI, "1", "SeriesInstanceUID"},
	{0x0020, 0x0010}: {VRSH, "1", "StudyID"},
	{0x0020, 0x0011}: {VRIS, "1", "SeriesNumber"},
	{0x0020, 0x0013}: {VRIS, "1", "InstanceNumber"},
	{0x7FE0, 0x0010}: {VROW, "1", "PixelData"},

	// File-meta (group 0002) essentials, for completeness.
	{0x0002, 0x0000}: {VRUL, "1", "FileMetaInformationGroupLength"},
	{0x0002, 0x0001}: {VROB, "1", "FileMetaInformationVersion"},
	{0x0002, 0x0002}: {VRUI, "1", "MediaStorageSOPClassUID"},
	{0x0002, 0x0003}: {VRUI, "1", "MediaStorageSOPInstanceUID"},
	{0x0002, 0x0010}: {VRUI, "1", "TransferSyntaxUID"},
	{0x0002, 0x0012}: {VRUI, "1", "ImplementationClassUID"},
	{0x0002, 0x0013}: {VRSH, "1", "ImplementationVersionName"},
}

// RegisterDictEntry adds or overrides a data dictionary entry. Safe to call at
// init time; not safe for concurrent use with lookups.
func RegisterDictEntry(tag Tag, entry DictEntry) {
	dictionary[tag] = entry
}

// LookupEntry returns the dictionary entry for a tag, if known.
func LookupEntry(tag Tag) (DictEntry, bool) {
	e, ok := dictionary[tag]
	return e, ok
}

// LookupVR returns the VR for a tag, resolving unknown tags as follows:
// group-length elements are UL; item/delimitation tags have no VR (UN);
// everything else defaults to UN (Unknown), which is read as raw bytes.
func LookupVR(tag Tag) VR {
	if e, ok := dictionary[tag]; ok {
		return e.VR
	}
	if tag.IsGroupLength() {
		return VRUL
	}
	return VRUN
}
