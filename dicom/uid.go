// SPDX-License-Identifier: Apache-2.0

package dicom

// Well-known DICOM UIDs used by the networking layer.

// Transfer Syntax UIDs (PS3.5 Annex A / PS3.6).
const (
	ImplicitVRLittleEndian = "1.2.840.10008.1.2"
	ExplicitVRLittleEndian = "1.2.840.10008.1.2.1"
	DeflatedExplicitVRLE   = "1.2.840.10008.1.2.1.99"
	ExplicitVRBigEndian    = "1.2.840.10008.1.2.2" // retired
)

// Application context (PS3.7 §A.2.1).
const DICOMApplicationContextName = "1.2.840.10008.3.1.1.1"

// Verification SOP Class (PS3.4 Annex A) — used by C-ECHO.
const VerificationSOPClass = "1.2.840.10008.1.1"

// Implementation identity advertised by this library in the A-ASSOCIATE
// User Information sub-items. The class UID lives in a private/example arc;
// adopters may override it via the AE configuration.
const (
	GoDICOMImplementationClassUID    = "1.2.826.0.1.3680043.10.1337.1"
	GoDICOMImplementationVersionName = "GODICOM_0_1"
)

// TransferSyntaxName returns a friendly name for a transfer syntax UID, or the
// UID itself if unknown.
func TransferSyntaxName(uid string) string {
	switch uid {
	case ImplicitVRLittleEndian:
		return "Implicit VR Little Endian"
	case ExplicitVRLittleEndian:
		return "Explicit VR Little Endian"
	case DeflatedExplicitVRLE:
		return "Deflated Explicit VR Little Endian"
	case ExplicitVRBigEndian:
		return "Explicit VR Big Endian"
	default:
		return uid
	}
}

// IsImplicitVR reports whether the transfer syntax UID denotes Implicit VR.
func IsImplicitVR(transferSyntax string) bool {
	return transferSyntax == ImplicitVRLittleEndian
}
