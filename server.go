package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

const (
	DefaultDuration = 5 * time.Minute // время жизни по умолчанию.
)

// Server описывает серверные подключения клиентов.
type Server struct {
	Addr        string        // TCP-адрес и порт сервера
	Duration    time.Duration // время жизни неактивного соединения
	TLSConfig   *tls.Config   // конфигурация TLS, которая используется ListenAndServeTLS
	ErrorLog    *log.Logger   // вывод лога (если не определен, то используется стандартный)
	connections *List         // информация об установленных соединениях
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
	srv.logf("Listen %s...", srv.Addr)
	if srv.Duration == 0 {
		srv.Duration = DefaultDuration // по умолчанию время жизни -- 5 минут
	}
	if srv.connections == nil {
		srv.connections = NewList(srv.Duration) // инициализируем хранилище информации о соединениях
	}
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
		if id != "" {
			srv.connections.Update(id) // обновляем время последней активности клиента
		}
		if err != nil {
			conn.Close() // закрываем соединение после любой ошибки
			return
		}
		message = strings.TrimSpace(message) // избавляемся от лишних пробелов
		var (
			splits = strings.SplitN(message, " ", 2) // отделяем команду от параметров
			cmd    = strings.ToUpper(splits[0])      // приводим команду к верхнему регистру
			param  string                            // параметр
		)
		if len(splits) > 1 {
			param = strings.TrimSpace(splits[1]) // сохраняем параметр, если он есть
		}
		// обрабатываем команды
		switch cmd {
		case CONNECT: // подключение
			if id == "" {
				if param != "" {
					id = param
					srv.connections.Add(id, addr)
					fmt.Fprintln(conn, OK, "Connected")
				} else {
					fmt.Fprintln(conn, ERROR, "Empty id")
				}
			} else {
				fmt.Fprintln(conn, ERROR, "Already connected")
			}
		case STATUS: // изменение текста статуса
			if id != "" {
				srv.connections.SetStatus(id, param)
				fmt.Fprintln(conn, OK, "Status changed")
			} else {
				fmt.Fprintln(conn, ERROR, "Not connected")
			}
		case INFO: // запрос информации о соединении
			if info := srv.connections.Info(param); info != nil &&
				time.Since(info.updated) < srv.Duration {
				fmt.Fprintln(conn, OK, INFO, param, info.String())
			} else {
				fmt.Fprintln(conn, OK, "Not found", param)
			}
		case PING: // поддержка соединения
			fmt.Fprintln(conn, OK, PONG, param)
		case DISCONNECT: // закрытие соединения
			fmt.Fprintln(conn, OK, "Disconnected")
			conn.Close() // закрываем соединение
			return       // больше нечего делать
		default: // неизвестная команда
			fmt.Fprintln(conn, OK, "Ignored command", cmd)
		}
	}
}
