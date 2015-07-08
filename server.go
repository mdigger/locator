package main

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

var timeout = time.Duration(2 * time.Minute)

// Server описывает серверные подключения клиентов.
type Server struct {
	Addr        string // TCP-адрес и порт сервера
	connections *List  // информация об установленных соединениях
}

func NewServer(connections *List) *Server {
	if connections == nil {
		connections = NewList() // инициализируем хранилище информации о соединениях
	}
	return &Server{
		connections: connections,
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
	log.Printf("Listen %s...", srv.Addr)
	if srv.connections == nil {
		srv.connections = NewList() // инициализируем хранилище информации о соединениях
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
				log.Printf("Accept error: %v; retrying in %v", err, tempDelay)
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

func Send(addr string, conn net.Conn, params ...string) error {
	msg := strings.Join(params, " ") + "\n"
	log.Printf("%s <- %s", addr, msg)
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	_, err := io.WriteString(conn, msg)
	return err
}

// servConn обрабатывает удаленное соединение.
func (srv *Server) servConn(conn net.Conn) {
	var (
		reader = bufio.NewReader(conn)      // буфер для чтения команд
		addr   = conn.RemoteAddr().String() // адрес удаленного сервера
		id     string                       // уникальный идентификатор соединения
	)
	log.Printf("%s <- connected", addr) // выводим информацию об установленно соединении
	defer func() {
		if id != "" {
			srv.connections.Remove(id)
		}
		conn.Close()                                  // закрываем соединение после любой ошибки
		log.Printf("%s -> disconnected %q", addr, id) // выводим информацию о закрытии соединения
		log.Println("!:", srv.connections.List())
	}()
	// читаем команды до тех пор, пока соединение не будет закрыто
	for {
		if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			log.Println(addr, "ERROR:", err.Error())
			return
		}
		message, err := reader.ReadString('\n') // читаем команду до конца строки
		if err != nil {
			log.Println(addr, "ERROR:", err.Error())
			return
		}
		if id != "" {
			srv.connections.Update(id) // обновляем время последней активности клиента
		}
		if len(message) > 256 {
			continue // игнорируем слишком длинные заголовки
		}
		message = strings.TrimSpace(message)  // избавляемся от лишних пробелов
		log.Printf("%s -> %s", addr, message) // выводим информацию о запросе
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
			if param != "" {
				var addr2 string
				if idx := strings.IndexRune(param, ' '); idx > 1 {
					id = param[:idx]
					addr2 = param[idx:]
					log.Printf("+ ADD 1: id - %q [%x], addr2: %q", id, id, addr2)
				} else {
					id = param
					log.Printf("+ ADD 2: id - %q [%x], addr2: %q", id, id, addr2)
				}
				srv.connections.Add(conn, id, addr, addr2)
				if err := Send(addr, conn, OK, cmd, id, addr); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
				log.Println("!:", srv.connections.List())
			} else {
				if err := Send(addr, conn, ERROR, cmd, "empty id"); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
			}
		case STATUS: // изменение текста статуса
			if id != "" {
				srv.connections.SetStatus(id, param)
				if err := Send(addr, conn, OK, cmd, param); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
			} else {
				if err := Send(addr, conn, ERROR, cmd, "not connected"); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
			}
		case INFO: // запрос информации о соединении
			// log.Printf("# INFO: %q", param)
			if info := srv.connections.Info(param); info != nil && info.conn != nil && time.Since(info.updated) < timeout {
				// log.Printf("# INFO: %q CONNECTED", param)
				if err := Send(info.addr, info.conn, PING, id); err != nil {
					log.Println(info.addr, "ERROR:", err.Error())
					srv.connections.Remove(param)
					if err := Send(addr, conn, ERROR, cmd, param, "not found"); err != nil {
						log.Println(addr, "ERROR:", err.Error())
						return // больше нечего делать
					}
					continue
				}
				// log.Printf("# INFO: %q CONNECTED 2", param)
				if err := Send(addr, conn, OK, cmd, param, info.String()); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
			} else {
				if info == nil {
					log.Printf("# INFO: %q NOT CONNECTED", param)
				} else if info.conn == nil {
					log.Printf("# INFO: %q CONNECTION IS NIL", param)
				} else if time.Since(info.updated) >= timeout {
					log.Printf("# INFO: %q CONNECTION TIMEOUT", param)
				} else {
					log.Printf("# INFO: %q CONNECTION UNKNOWN ERROR", param)
				}
				if err := Send(addr, conn, ERROR, cmd, param, "not found"); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
			}
			log.Printf("# INFO: %q END", param)
		case PING: // поддержка соединения
			if err := Send(addr, conn, OK, cmd, param); err != nil {
				log.Println(addr, "ERROR:", err.Error())
				return // больше нечего делать
			}
		case DISCONNECT: // закрытие соединения
			Send(addr, conn, OK, cmd)
			return // больше нечего делать
		case TO:
			if param == "" {
				if err := Send(addr, conn, ERROR, cmd, "empty TO"); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
				reader.Reset(conn)
				continue
			}
			to := srv.connections.Info(param)
			if to == nil || to.conn == nil || time.Since(to.updated) > timeout {
				if err := Send(addr, conn, ERROR, cmd, param, "not connected"); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
				reader.Reset(conn)
				continue
			}
			var length int32
			if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
				if err := Send(addr, conn, ERROR, cmd, err.Error()); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
				reader.Reset(conn)
				continue
			}
			// log.Println(addr, "Length %d:", length-4)
			if err := Send(to.addr, to.conn, "FROM", id); err != nil {
				if err := Send(addr, conn, ERROR, cmd, err.Error()); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
				reader.Reset(conn)
				srv.connections.Remove(param)
				continue
			}
			if err := binary.Write(to.conn, binary.LittleEndian, length); err != nil {
				if err := Send(addr, conn, ERROR, cmd, err.Error()); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
				reader.Reset(conn)
				srv.connections.Remove(param)
				continue
			}
			if err := to.conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
				log.Println(addr, "ERROR Write Deadline:", err.Error())
				if err := Send(addr, conn, ERROR, cmd, err.Error()); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
				reader.Reset(conn)
				srv.connections.Remove(param)
				continue
			}
			if n, err := io.CopyN(to.conn, reader, int64(length-4)); err != nil {
				if err := Send(addr, conn, ERROR, cmd, err.Error()); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
				reader.Reset(conn)
				srv.connections.Remove(param)
				continue
			} else {
				log.Printf("transform from %q to %q completed [%d]", id, param, n)
				if err := Send(addr, conn, OK, cmd, param); err != nil {
					log.Println(addr, "ERROR:", err.Error())
					return // больше нечего делать
				}
			}

		default: // неизвестная команда
			reader.Reset(conn)
		}
	}
}
