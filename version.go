// SPDX-License-Identifier: Apache-2.0

package godicom

// Version is the semantic version of the godicom library and the single source
// of truth for releases. Bump it, then tag the commit `vX.Y.Z`; the release
// workflow verifies the tag matches this constant.
const Version = "1.0.0"

// VersionString returns the version, appending "+<commit>" when a non-empty
// build commit is supplied (commands inject it via -ldflags "-X main.commit=…").
func VersionString(commit string) string {
	if commit == "" {
		return "v" + Version
	}
	return "v" + Version + "+" + commit
}
