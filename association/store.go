// SPDX-License-Identifier: Apache-2.0

package association

import (
	"fmt"

	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
)

// Data-set SOP identity tags (PS3.3).
var (
	tagSOPClassUID    = dicom.Tag{Group: 0x0008, Element: 0x0016}
	tagSOPInstanceUID = dicom.Tag{Group: 0x0008, Element: 0x0018}
)

// SendCStore transmits a data set to the peer as a C-STORE request and returns
// the response status. The SOP Class and Instance UIDs are read from the data
// set's (0008,0016) and (0008,0018); a presentation context accepted for that
// SOP class must exist, and the data set is encoded in that context's
// negotiated transfer syntax.
func (a *Association) SendCStore(ds *dicom.DataSet) (dimse.Status, error) {
	sopClass, _ := ds.GetString(tagSOPClassUID)
	if sopClass == "" {
		return 0, fmt.Errorf("association: data set missing SOP Class UID (0008,0016)")
	}
	sopInstance, _ := ds.GetString(tagSOPInstanceUID)

	ctx, ok := a.contextForSyntax(sopClass)
	if !ok {
		return 0, fmt.Errorf("association: no accepted presentation context for SOP class %s", sopClass)
	}

	rq := &dimse.CStoreRequest{
		MessageID:              a.nextMessageID(),
		AffectedSOPClassUID:    sopClass,
		AffectedSOPInstanceUID: sopInstance,
		Priority:               dimse.PriorityMedium,
	}
	if err := a.sendMessage(ctx, rq, ds); err != nil {
		return 0, err
	}
	_, msg, _, err := a.readMessage()
	if err != nil {
		return 0, err
	}
	rsp, ok := msg.(*dimse.CStoreResponse)
	if !ok {
		return 0, fmt.Errorf("association: expected C-STORE-RSP, got %s", msg.CommandField())
	}
	return rsp.Status, nil
}
