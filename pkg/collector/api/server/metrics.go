package apiserver

import "github.com/prometheus/client_golang/prometheus/collectors"

func (s *Server) registerMetrics() {
	s.reg.MustRegister(collectors.NewGoCollector())
	s.reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}
