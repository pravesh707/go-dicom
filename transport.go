// SPDX-License-Identifier: Apache-2.0

package godicom

import "net"

// Transport abstracts how an AE opens outbound connections and listens for
// inbound ones. The default is TCP; tests or embedders can inject an in-memory
// or TLS transport without changing the AE (Dependency Inversion). Both the
// client and server depend on this interface, not on net directly.
type Transport interface {
	Dial(addr string) (net.Conn, error)
	Listen(addr string) (net.Listener, error)
}

// TCPTransport is the default Transport, using ordinary TCP sockets.
type TCPTransport struct{}

func (TCPTransport) Dial(addr string) (net.Conn, error)       { return net.Dial("tcp", addr) }
func (TCPTransport) Listen(addr string) (net.Listener, error) { return net.Listen("tcp", addr) }
