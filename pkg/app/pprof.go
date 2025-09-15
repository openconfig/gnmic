package app

import (
	"net/http"
	_ "net/http/pprof" //nolint:gosec // Import for pprof, only enabled via CLI flag
	"time"
)

type pprofServer struct {
	err chan error
}

func newPprofServer() *pprofServer {
	return &pprofServer{
		err: make(chan error, 1),
	}
}

func (p *pprofServer) Start(address string) {
	go func() {
		server := &http.Server{
			Addr:              address,
			ReadHeaderTimeout: 10 * time.Second,
		}

		if err := server.ListenAndServe(); err != nil {
			p.err <- err
		}
		close(p.err)
	}()
}

func (p *pprofServer) ErrChan() <-chan error {
	return p.err
}
