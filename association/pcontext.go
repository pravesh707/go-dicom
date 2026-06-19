// SPDX-License-Identifier: Apache-2.0

// Package association implements the DICOM Upper Layer association: ACSE
// negotiation (requestor and acceptor), a pragmatic realization of the PS3.8
// state machine, presentation-context negotiation, PDV fragmentation /
// reassembly over P-DATA-TF, and DIMSE message exchange. It is the rough
// equivalent of association, acse, dul and dimse modules.
package association

import "github.com/pravesh707/go-dicom/pdu"

// RequestedContext is an abstract syntax that a requestor (SCU) proposes,
// together with the transfer syntaxes it is willing to use, in preference
// order.
type RequestedContext struct {
	AbstractSyntax   string
	TransferSyntaxes []string
}

// SupportedContext is an abstract syntax that an acceptor (SCP) will accept,
// together with the transfer syntaxes it supports.
type SupportedContext struct {
	AbstractSyntax   string
	TransferSyntaxes []string
}

// AcceptedContext is the negotiated outcome for one presentation context that
// was accepted by the acceptor.
type AcceptedContext struct {
	ID             byte
	AbstractSyntax string
	TransferSyntax string
}

// negotiate matches the requestor's proposed presentation contexts against the
// acceptor's supported contexts. It returns the accept-side presentation
// context items and a map of accepted contexts keyed by presentation context
// ID. Contexts whose abstract syntax is unsupported, or which share no transfer
// syntax, are rejected with the appropriate reason.
func negotiate(requested []pdu.PresentationContextRQ, supported []SupportedContext) ([]pdu.PresentationContextAC, map[byte]AcceptedContext) {
	supportedBySyntax := make(map[string]SupportedContext, len(supported))
	for _, s := range supported {
		supportedBySyntax[s.AbstractSyntax] = s
	}

	var acItems []pdu.PresentationContextAC
	accepted := make(map[byte]AcceptedContext)

	for _, rq := range requested {
		sup, ok := supportedBySyntax[rq.AbstractSyntax]
		if !ok {
			acItems = append(acItems, pdu.PresentationContextAC{
				ID: rq.ID, Result: pdu.PCAbstractSyntaxNotSup,
			})
			continue
		}
		ts := firstCommon(rq.TransferSyntaxes, sup.TransferSyntaxes)
		if ts == "" {
			acItems = append(acItems, pdu.PresentationContextAC{
				ID: rq.ID, Result: pdu.PCTransferSyntaxNotSup,
			})
			continue
		}
		acItems = append(acItems, pdu.PresentationContextAC{
			ID: rq.ID, Result: pdu.PCAccepted, TransferSyntax: ts,
		})
		accepted[rq.ID] = AcceptedContext{
			ID: rq.ID, AbstractSyntax: rq.AbstractSyntax, TransferSyntax: ts,
		}
	}
	return acItems, accepted
}

// firstCommon returns the first element of prefer that also appears in avail,
// preserving the requestor's preference order.
func firstCommon(prefer, avail []string) string {
	set := make(map[string]bool, len(avail))
	for _, a := range avail {
		set[a] = true
	}
	for _, p := range prefer {
		if set[p] {
			return p
		}
	}
	return ""
}
