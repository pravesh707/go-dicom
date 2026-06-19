// SPDX-License-Identifier: Apache-2.0

package dimse

// CommandField identifies the DIMSE service primitive (PS3.7 §9.1, §10.1).
// Response codes are the request code OR-ed with 0x8000.
type CommandField uint16

const (
	CStoreRQ        CommandField = 0x0001
	CStoreRSP       CommandField = 0x8001
	CGetRQ          CommandField = 0x0010
	CGetRSP         CommandField = 0x8010
	CFindRQ         CommandField = 0x0020
	CFindRSP        CommandField = 0x8020
	CMoveRQ         CommandField = 0x0021
	CMoveRSP        CommandField = 0x8021
	CEchoRQ         CommandField = 0x0030
	CEchoRSP        CommandField = 0x8030
	CCancelRQ       CommandField = 0x0FFF
	NEventReportRQ  CommandField = 0x0100
	NEventReportRSP CommandField = 0x8100
	NGetRQ          CommandField = 0x0110
	NGetRSP         CommandField = 0x8110
	NSetRQ          CommandField = 0x0120
	NSetRSP         CommandField = 0x8120
	NActionRQ       CommandField = 0x0130
	NActionRSP      CommandField = 0x8130
	NCreateRQ       CommandField = 0x0140
	NCreateRSP      CommandField = 0x8140
	NDeleteRQ       CommandField = 0x0150
	NDeleteRSP      CommandField = 0x8150
)

// Priority is the DIMSE service priority (PS3.7 §10.3).
type Priority uint16

const (
	PriorityMedium Priority = 0x0000
	PriorityHigh   Priority = 0x0001
	PriorityLow    Priority = 0x0002
)

// commandDataSetType values for the (0000,0800) element.
const (
	dataSetTypeAbsent  = 0x0101 // no data set follows the command
	dataSetTypePresent = 0x0000 // a data set follows (any non-0x0101 value)
)

func (cf CommandField) String() string {
	switch cf {
	case CStoreRQ:
		return "C-STORE-RQ"
	case CStoreRSP:
		return "C-STORE-RSP"
	case CGetRQ:
		return "C-GET-RQ"
	case CGetRSP:
		return "C-GET-RSP"
	case CFindRQ:
		return "C-FIND-RQ"
	case CFindRSP:
		return "C-FIND-RSP"
	case CMoveRQ:
		return "C-MOVE-RQ"
	case CMoveRSP:
		return "C-MOVE-RSP"
	case CEchoRQ:
		return "C-ECHO-RQ"
	case CEchoRSP:
		return "C-ECHO-RSP"
	case CCancelRQ:
		return "C-CANCEL-RQ"
	case NEventReportRQ:
		return "N-EVENT-REPORT-RQ"
	case NEventReportRSP:
		return "N-EVENT-REPORT-RSP"
	case NGetRQ:
		return "N-GET-RQ"
	case NGetRSP:
		return "N-GET-RSP"
	case NSetRQ:
		return "N-SET-RQ"
	case NSetRSP:
		return "N-SET-RSP"
	case NActionRQ:
		return "N-ACTION-RQ"
	case NActionRSP:
		return "N-ACTION-RSP"
	case NCreateRQ:
		return "N-CREATE-RQ"
	case NCreateRSP:
		return "N-CREATE-RSP"
	case NDeleteRQ:
		return "N-DELETE-RQ"
	case NDeleteRSP:
		return "N-DELETE-RSP"
	default:
		return "UNKNOWN"
	}
}
