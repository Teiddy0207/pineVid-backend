package httpserver

import (
	"net"
	"strings"
	"time"
)

// Option -.
type Option func(*Server)

// Port -.
func Port(port string) Option {
	return func(s *Server) {
		s.address = net.JoinHostPort("", port)
	}
}

// Prefork -.
func Prefork(prefork bool) Option {
	return func(s *Server) {
		s.prefork = prefork
	}
}

// ReadTimeout -.
func ReadTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.readTimeout = timeout
	}
}

// WriteTimeout -.
func WriteTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.writeTimeout = timeout
	}
}

// ShutdownTimeout -.
func ShutdownTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.shutdownTimeout = timeout
	}
}

// TrustedProxies sets the comma-separated reverse-proxy IPs/CIDRs whose
// X-Forwarded-For header is trusted. Empty = no proxy.
func TrustedProxies(list string) Option {
	return func(s *Server) {
		for _, p := range strings.Split(list, ",") {
			if p = strings.TrimSpace(p); p != "" {
				s.trustedProxies = append(s.trustedProxies, p)
			}
		}
	}
}
