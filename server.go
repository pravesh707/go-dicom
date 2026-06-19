// SPDX-License-Identifier: Apache-2.0

package godicom

import (
	"net"
	"sync"

	"github.com/pravesh707/go-dicom/association"
)

// Server is a running DICOM SCP. Each inbound association is negotiated and
// served on its own goroutine, so throughput scales across CPU cores.
type Server struct {
	ae       *AE
	listener net.Listener
	bindings []HandlerBinding

	wg      sync.WaitGroup
	mu      sync.Mutex
	closing bool
}

// StartServer begins listening on addr ("host:port", e.g. ":11112") and serves
// associations in the background using the supplied event handlers. It returns
// once the listener is open; use Server.Addr to discover the bound address
// (useful when addr uses port 0) and Server.Shutdown to stop.
func (ae *AE) StartServer(addr string, bindings []HandlerBinding) (*Server, error) {
	ln, err := ae.transport.Listen(addr)
	if err != nil {
		return nil, err
	}
	s := &Server{ae: ae, listener: ln, bindings: bindings}
	s.wg.Add(1)
	go s.acceptLoop()
	return s, nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closing := s.closing
			s.mu.Unlock()
			if closing {
				return
			}
			return
		}
		s.wg.Add(1)
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer s.wg.Done()
	assoc, err := association.Accept(conn, association.AcceptParams{
		AETitle:                   s.ae.AETitle,
		SupportedContexts:         s.ae.supported,
		MaximumLength:             s.ae.MaximumLength,
		ImplementationClassUID:    s.ae.ImplementationClassUID,
		ImplementationVersionName: s.ae.ImplementationVersionName,
		RequireCalledAET:          s.ae.RequireCalledAET,
	})
	if err != nil {
		return
	}
	_ = assoc.Serve(s.bindings)
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() net.Addr { return s.listener.Addr() }

// Shutdown stops accepting new associations and waits for in-flight ones to
// finish.
func (s *Server) Shutdown() error {
	s.mu.Lock()
	s.closing = true
	s.mu.Unlock()
	err := s.listener.Close()
	s.wg.Wait()
	return err
}
