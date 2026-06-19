// SPDX-License-Identifier: Apache-2.0

package godicom

// Option customises an AE at construction time (functional options pattern).
type Option func(*AE)

// WithMaximumLength sets the maximum PDU length the AE advertises (bytes).
func WithMaximumLength(n uint32) Option {
	return func(a *AE) { a.MaximumLength = n }
}

// WithImplementationClassUID overrides the advertised Implementation Class UID.
func WithImplementationClassUID(uid string) Option {
	return func(a *AE) { a.ImplementationClassUID = uid }
}

// WithImplementationVersionName overrides the advertised Implementation Version
// Name.
func WithImplementationVersionName(name string) Option {
	return func(a *AE) { a.ImplementationVersionName = name }
}

// WithTransport injects a custom Transport (e.g. TLS or in-memory for tests).
func WithTransport(t Transport) Option {
	return func(a *AE) { a.transport = t }
}

// RequireCalledAETitle makes a server reject associations whose Called AE Title
// does not match this AE's title.
func RequireCalledAETitle() Option {
	return func(a *AE) { a.RequireCalledAET = true }
}
