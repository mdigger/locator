package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// Server описывает серверные подключения клиентов.
type Server struct {
	Addr      string      // TCP address to listen on, ":9000" if empty
	TLSConfig *tls.Config // optional TLS config, used by ListenAndServeTLS
	// ErrorLog specifies an optional logger for errors accepting connections and unexpected
	// behavior from handlers. If nil, logging goes to os.Stderr via the log package's
	// standard logger.
	ErrorLog *log.Logger

	connections map[string]*Conn
	mu          sync.RWMutex
}

// ListenAndServe listens on the TCP network address srv.Addr and then
// calls Serve to handle requests on incoming connections.
func (srv *Server) ListenAndServe() error {
	if srv.Addr == "" {
		srv.Addr = ":9080"
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}

// ListenAndServeTLS запускает TLS-версию сервера
func (srv *Server) ListenAndServeTLS(certFile, keyFile string) error {
	if srv.Addr == "" {
		srv.Addr = ":9403"
	}
	config := &tls.Config{}
	if srv.TLSConfig != nil {
		*config = *srv.TLSConfig
	}

	var err error
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}

	tlsListener := tls.NewListener(ln, config)
	return srv.Serve(tlsListener)
}

// Serve accepts incoming connections on the Listener l, creating a
// new service goroutine for each.
func (srv *Server) Serve(l net.Listener) error {
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		conn, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				srv.logf("Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			l.Close()
			return err
		}
		tempDelay = 0
		c, err := srv.newConn(conn)
		if err != nil {
			continue
		}
		go c.serve()
	}
}

func (srv *Server) newConn(conn net.Conn) (*Conn, error) {
	var connection = &Conn{
		server: srv,
		conn:   conn,
		addr:   conn.RemoteAddr().String(),
		reader: bufio.NewReader(conn),
	}

	var cmd, params, err = connection.Read()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if cmd != CONNECT {
		connection.Send(DISCONNECT, "Incorrect CONNECT command")
		conn.Close()
		return nil, errors.New("incorrect connect command")
	}
	if idx := strings.IndexRune(params, ' '); idx > 0 {
		connection.id = params[:idx]
		connection.status = params[idx:]
	} else {
		connection.id = params
	}
	if connection.id == "" {
		connection.Send(DISCONNECT, "Empty CONNECT client_id")
		conn.Close()
		return nil, errors.New("empty client_id")
	}
	srv.mu.Lock()
	if srv.connections == nil {
		srv.connections = make(map[string]*Conn)
	}
	if _, ok := srv.connections[connection.id]; ok {
		connection.Send(DISCONNECT, fmt.Sprintf("Client with id %q already connected", connection.id))
		conn.Close()
		srv.mu.Unlock()
		return nil, errors.New("duplicate client_id")
	}
	srv.connections[connection.id] = connection
	srv.logf("Connect: %s [%s]", connection.id, connection.addr)
	srv.mu.Unlock()
	connection.Send(OK, "Connected")
	return connection, nil
}

func (srv *Server) logf(format string, args ...interface{}) {
	if srv.ErrorLog != nil {
		srv.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// Shutdown отправляет всем клиентам сообщение о закрытии.
func (srv *Server) Shutdown() {
	srv.mu.RLock()
	for _, connection := range srv.connections {
		connection.Send(SHUTDOWN)
	}
	srv.mu.RUnlock()
	srv.mu.Lock()
	srv.connections = nil
	srv.mu.Unlock()
	srv.logf("Server closed")
}

// Disconnect удаляет информацию о соединении.
func (srv *Server) Disconnect(id string) {
	srv.mu.Lock()
	delete(srv.connections, id)
	srv.mu.Unlock()
	srv.logf("Disconnect: %s", id)
}

// Info возвращает информацию о соединении с указанным идентификатором.
func (srv *Server) Info(id string) string {
	srv.mu.RLock()
	connection, ok := srv.connections[id]
	srv.mu.RUnlock()
	if !ok {
		return "NOT FOUND"
	}
	return connection.Info()
}
