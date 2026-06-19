// SPDX-License-Identifier: Apache-2.0

package association

import (
	"fmt"

	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
)

// SendCFind issues a C-FIND query on the accepted context for sopClass with the
// given identifier (query keys), collecting every matching identifier until the
// final response. It returns the matches and the final status.
func (a *Association) SendCFind(sopClass string, query *dicom.DataSet) ([]*dicom.DataSet, dimse.Status, error) {
	ctx, ok := a.contextForSyntax(sopClass)
	if !ok {
		return nil, 0, fmt.Errorf("association: no accepted presentation context for %s", sopClass)
	}
	rq := &dimse.CFindRequest{
		MessageID:           a.nextMessageID(),
		AffectedSOPClassUID: sopClass,
		Priority:            dimse.PriorityMedium,
		Identifier:          query,
	}
	if err := a.sendMessage(ctx, rq, query); err != nil {
		return nil, 0, err
	}

	var matches []*dicom.DataSet
	for {
		_, msg, ds, err := a.readMessage()
		if err != nil {
			return matches, 0, err
		}
		rsp, ok := msg.(*dimse.CFindResponse)
		if !ok {
			return matches, 0, fmt.Errorf("association: expected C-FIND-RSP, got %s", msg.CommandField())
		}
		if rsp.Status.IsPending() {
			matches = append(matches, ds)
			continue
		}
		return matches, rsp.Status, nil
	}
}
