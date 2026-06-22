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
	active  map[*association.Association]struct{}
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
	s := &Server{
		ae:       ae,
		listener: ln,
		bindings: bindings,
		active:   make(map[*association.Association]struct{}),
	}
	s.wg.Add(1)
	go s.acceptLoop()
	return s, nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed (Shutdown) or a fatal accept error
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
		MoveResolver:              s.ae.moveResolver(),
		MoveStorageContexts:       s.ae.moveStorageContexts(),
	})
	if err != nil {
		return
	}

	// Register the association so Shutdown can force-close it. If a shutdown
	// has already begun, close immediately instead of serving.
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		assoc.Close()
		return
	}
	s.active[assoc] = struct{}{}
	s.mu.Unlock()

	_ = assoc.Serve(s.bindings)

	s.mu.Lock()
	delete(s.active, assoc)
	s.mu.Unlock()
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() net.Addr { return s.listener.Addr() }

// Shutdown stops accepting new associations, force-closes any still-open ones,
// and waits for all serving goroutines to finish. It never blocks indefinitely
// on an idle peer.
func (s *Server) Shutdown() error {
	s.mu.Lock()
	s.closing = true
	active := make([]*association.Association, 0, len(s.active))
	for a := range s.active {
		active = append(active, a)
	}
	s.mu.Unlock()

	err := s.listener.Close()
	for _, a := range active {
		a.Close()
	}
	s.wg.Wait()
	return err
}
