package server

import (
	"fmt"
	"net"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

type Server struct {
	logr.Logger
	clients map[byte]net.Conn
}

func New() *Server {
	z, _ := zap.NewDevelopment()
	l := zapr.NewLogger(z)
	return &Server{Logger: l, clients: make(map[byte]net.Conn)}
}

func (s *Server) ListenAndServe() {
	l, err := net.Listen("tcp4", "0.0.0.0:443")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	s.Info("Listening.", "addr", l.Addr().String())
	for {
		c, err := l.Accept()
		if err != nil {
			s.Error(err, "error accepting connection")
			continue
		}
		s.Info("connected", "remote", c.RemoteAddr().String(), "conn", fmt.Sprintf("%+v", c))
		if err := s.establishSession(c); err != nil {
			c.Close()
			continue
		}
		s.Info("established session", "remote", c.RemoteAddr().String(), "conn", fmt.Sprintf("%+v", c))
		go s.handleConnection(c)
	}
}

func (s *Server) establishSession(c net.Conn) error {
	buf := make([]byte, 64)
	for {
		n, err := c.Read(buf)
		if err != nil {
			return err
		}
		if n < 9 || string(buf[:8]) != "hubproto" {
			return fmt.Errorf("non-client request")
		}
		s.clients[buf[8]] = c
		s.Info("establishing session", "client-id", buf[8], "client-conn", fmt.Sprintf("%+v", c))
		c.Write([]byte("OK"))
		return nil
	}
}

func (s *Server) handleConnection(c net.Conn) {
	defer c.Close()
	s.Info("handling connection", "remote", c.RemoteAddr().String(), "conn", fmt.Sprintf("%+v", c))
	buf := make([]byte, 1536)
	for {
		n, err := c.Read(buf)
		// Probably connection closed, so EOF and break
		if err != nil {
			break
		}
		if n < 9 || string(buf[:8]) != "hubproto" {
			continue
		}
		s.Info("authHeader read", "authHeader", fmt.Sprint(buf[:9]), "header", fmt.Sprint(buf[9:29]), "bodyStart", fmt.Sprint(buf[29:93]))
		s.Info("message authorized", "length", n, "peer-id", buf[8])
		p, ok := s.clients[buf[8]]
		if !ok {
			continue
		}
		p.Write(buf[9:n])
		s.Info("got message", "source", fmt.Sprintf("%+v", c), "dest", fmt.Sprintf("%+v", p), "length", n)
	}
}
