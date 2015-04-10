package main

import (
	"bufio"
	"crypto/tls"
	"log"
	"net"
	"strings"
	"time"
)

// Server описывает серверные подключения клиентов.
type Server struct {
	Addr        string      // TCP-адрес и порт сервера
	TLSConfig   *tls.Config // опциональная  конфигурация TLS, которая используется ListenAndServeTLS
	ErrorLog    *log.Logger // вывод лога (если не определен, то используется стандартный)
	connections list        // информация об установленных соединениях
}

// logf выводит информацию в лог.
func (srv *Server) logf(format string, args ...interface{}) {
	if srv.ErrorLog != nil {
		srv.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// ListenAndServe запускает сервер. Если адрес сервера не указан, то используется порт :9000
func (srv *Server) ListenAndServe() error {
	if srv.Addr == "" {
		srv.Addr = ":9000"
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}

// ListenAndServeTLS запускает TLS-версию сервера. Если не указан адрес сервера, то используется
// порт :9001
func (srv *Server) ListenAndServeTLS(certFile, keyFile string) error {
	if srv.Addr == "" {
		srv.Addr = ":9001"
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

// Serve принимает входящее соединение и запускает в отдельном потоке его обработку.
func (srv *Server) Serve(l net.Listener) error {
	var tempDelay time.Duration // задержка до возврата ошибки
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
		go srv.servConn(conn) // запускаем обработку соединения
	}
}

// Поддерживаемые команды
const (
	CONNECT    = "CONNECT"
	DISCONNECT = "DISCONNECT"
	STATUS     = "STATUS"
	INFO       = "INFO"
	PING       = "PING"
	PONG       = "PONG"
	OK         = "OK"
	ERROR      = "ERROR"
	SHUTDOWN   = "SHUTDOWN"
)

// servConn обрабатывает удаленное соединение.
func (srv *Server) servConn(conn net.Conn) {
	var (
		reader = bufio.NewReader(conn)      // буфер для чтения команд
		addr   = conn.RemoteAddr().String() // адрес удаленного сервера
		id     string                       // уникальный идентификатор соединения
	)
	// читаем команды до тех пор, пока соединение не будет закрыто
	for {
		message, err := reader.ReadString('\n') // читаем команду до конца строки
		if err != nil {
			return
		}
		message = strings.TrimSpace(message) // избавляемся от лишних пробелов
		var (
			splits = strings.SplitN(message, " ", 2) // отделяем команду от параметров
			cmd    = strings.ToUpper(splits[0])      // приводим команду к верхнему регистру
			param  string                            // параметр
		)
		if len(splits) > 1 {
			param = splits[1] // сохраняем параметр, если он есть
		}
		// обрабатываем команды
		switch cmd {
		case CONNECT:
		}
	}

	// var connection = &Conn{
	// 	server: srv,
	// 	conn:   conn,
	// 	addr:   conn.RemoteAddr().String(),
	// 	reader: bufio.NewReader(conn),
	// }

	// var cmd, params, err = connection.Read()
	// if err != nil {
	// 	conn.Close()
	// 	return nil, err
	// }
	// if cmd != CONNECT {
	// 	connection.Send(DISCONNECT, "Incorrect CONNECT command")
	// 	conn.Close()
	// 	return nil, errors.New("incorrect connect command")
	// }
	// if idx := strings.IndexRune(params, ' '); idx > 0 {
	// 	connection.id = params[:idx]
	// 	connection.status = params[idx:]
	// } else {
	// 	connection.id = params
	// }
	// if connection.id == "" {
	// 	connection.Send(DISCONNECT, "Empty CONNECT client_id")
	// 	conn.Close()
	// 	return nil, errors.New("empty client_id")
	// }
	// srv.mu.Lock()
	// if srv.connections == nil {
	// 	srv.connections = make(map[string]*Conn)
	// }
	// if _, ok := srv.connections[connection.id]; ok {
	// 	connection.Send(DISCONNECT, fmt.Sprintf("Client with id %q already connected", connection.id))
	// 	conn.Close()
	// 	srv.mu.Unlock()
	// 	return nil, errors.New("duplicate client_id")
	// }
	// srv.connections[connection.id] = connection
	// srv.logf("Connect: %s [%s]", connection.id, connection.addr)
	// srv.mu.Unlock()
	// connection.Send(OK, "Connected")
	// return connection, nil
}
