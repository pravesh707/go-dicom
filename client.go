// SPDX-License-Identifier: Apache-2.0

package godicom

import (
	"github.com/pravesh707/go-dicom/association"
)

// DefaultCalledAETitle is used when Associate is called without an explicit
// called AE title.
const DefaultCalledAETitle = "YOUR-SCP"

// Associate opens a TCP connection to addr ("host:port") and negotiates an
// association as an SCU, using the requested presentation contexts. The called
// AE title defaults to DefaultCalledAETitle.
func (ae *AE) Associate(addr string) (*Association, error) {
	return ae.AssociateAs(addr, DefaultCalledAETitle)
}

// AssociateAs is Associate with an explicit called AE title.
func (ae *AE) AssociateAs(addr, calledAETitle string) (*Association, error) {
	conn, err := ae.transport.Dial(addr)
	if err != nil {
		return nil, err
	}
	return association.Request(conn, association.RequestParams{
		CallingAETitle:            ae.AETitle,
		CalledAETitle:             calledAETitle,
		RequestedContexts:         ae.requested,
		MaximumLength:             ae.MaximumLength,
		ImplementationClassUID:    ae.ImplementationClassUID,
		ImplementationVersionName: ae.ImplementationVersionName,
	})
}
