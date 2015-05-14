package main

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

// Server описывает серверные подключения клиентов.
type Server struct {
	Addr        string        // TCP-адрес и порт сервера
	Duration    time.Duration // время жизни неактивного соединения (по умолчанию 5 минут)
	TLSConfig   *tls.Config   // конфигурация TLS, которая используется ListenAndServeTLS
	ErrorLog    *log.Logger   // вывод лога (если не определен, то используется стандартный)
	connections *List         // информация об установленных соединениях
	senders     map[string]net.Conn
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
	if srv.Duration < 30*time.Second {
		srv.Duration = 5 * time.Minute // по умолчанию время жизни -- 5 минут
	}
	if srv.connections == nil {
		srv.connections = NewList(srv.Duration) // инициализируем хранилище информации о соединениях
	}
	var tempDelay time.Duration // задержка до возврата ошибки
	if srv.senders == nil {
		srv.senders = make(map[string]net.Conn)
	}
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
	TO         = "TO"
)

// servConn обрабатывает удаленное соединение.
func (srv *Server) servConn(conn net.Conn) {
	var (
		reader = bufio.NewReader(conn)      // буфер для чтения команд
		addr   = conn.RemoteAddr().String() // адрес удаленного сервера
		id     string                       // уникальный идентификатор соединения
	)
	srv.logf("%s <- connected", addr)          // выводим информацию об установленно соединении
	defer srv.logf("%s -> disconnected", addr) // выводим информацию о закрытии соединения
	// читаем команды до тех пор, пока соединение не будет закрыто
	for {
		conn.SetDeadline(time.Now().Add(srv.Duration)) // устанавливаем время жизни по умолчанию
		message, err := reader.ReadString('\n')        // читаем команду до конца строки
		if id != "" {
			srv.connections.Update(id) // обновляем время последней активности клиента
		}
		if err != nil {
			srv.logf("DELETED: %s", id)
			delete(srv.senders, id)
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
		srv.logf("%s %s %s", addr, cmd, param) // выводим информацию о запросе
		switch cmd {
		case CONNECT: // подключение
			if id == "" {
				if param != "" {
					var addr2 string
					if idx := strings.IndexRune(param, ' '); idx > 1 {
						id = param[:idx]
						addr2 = param[idx:]
					} else {
						id = param
					}
					srv.connections.Add(id, addr, addr2)
					fmt.Fprintln(conn, OK, cmd, id, addr)
					srv.senders[id] = conn
					srv.logf("ADD: %s", id)
				} else {
					fmt.Fprintln(conn, ERROR, cmd, "empty id")
				}
			} else {
				fmt.Fprintln(conn, ERROR, cmd, "already connected")
			}
		case STATUS: // изменение текста статуса
			if id != "" {
				srv.connections.SetStatus(id, param)
				fmt.Fprintln(conn, OK, cmd, param)
			} else {
				fmt.Fprintln(conn, ERROR, cmd, "not connected")
			}
		case INFO: // запрос информации о соединении
			if info := srv.connections.Info(param); info != nil &&
				time.Since(info.updated) < srv.Duration {
				fmt.Fprintln(conn, OK, cmd, param, info.String())
			} else {
				fmt.Fprintln(conn, ERROR, cmd, param, "not found")
			}
		case PING: // поддержка соединения
			fmt.Fprintln(conn, OK, cmd, param)
		case DISCONNECT: // закрытие соединения
			fmt.Fprintln(conn, OK, cmd)
			srv.logf("DELETED: %s", id)
			delete(srv.senders, id)
			conn.Close() // закрываем соединение
			return       // больше нечего делать
		case TO:
			srv.logf("FROM: %s", id)
			if param == "" {
				fmt.Fprintln(conn, ERROR, cmd, "empty TO")
				reader.Reset(conn)
				continue
			}
			srv.logf("TO: %s", param)
			to, ok := srv.senders[param]
			if !ok || to == nil {
				srv.logf("TO: Error %s", "not connected")
				fmt.Fprintln(conn, ERROR, cmd, param, "not connected")
				reader.Reset(conn)
				continue
			}
			srv.logf("Sender found")
			var length int32
			if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
				srv.logf("TO: Error %s %s", "length read", err.Error())
				fmt.Fprintln(conn, ERROR, cmd, err.Error())
				reader.Reset(conn)
				continue
			}
			srv.logf("TO: Length %d", length)
			if _, err := fmt.Fprintln(to, "FROM", id); err != nil {
				srv.logf("TO: Error %s %s", "from send", err.Error())
				fmt.Fprintln(conn, ERROR, cmd, err.Error())
				reader.Reset(conn)
				continue
			}
			if err := binary.Write(to, binary.LittleEndian, length); err != nil {
				srv.logf("TO: Error %s %s", "length send", err.Error())
				fmt.Fprintln(conn, ERROR, cmd, err.Error())
				reader.Reset(conn)
				continue
			}
			if n, err := io.CopyN(to, reader, int64(length-4)); err != nil {
				srv.logf("TO: Error copy %s", err.Error())
				fmt.Fprintln(conn, ERROR, cmd, err.Error())
				reader.Reset(conn)
				continue
			} else {
				fmt.Fprintln(conn, OK, cmd, param)
				srv.logf("TO: Sended %d", n)
			}
			srv.logf("transform from %s to %s completed", id, param)
		default: // неизвестная команда
			fmt.Fprintln(conn, ERROR, cmd, "unknown command")
		}
	}
}
