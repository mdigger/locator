package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strings"
)

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

// Conn описывает информацию о подключенном к серверу клиенте.
type Conn struct {
	server *Server
	conn   net.Conn
	id     string
	status string
	addr   string
	reader *bufio.Reader
}

// Close закрывает соединение.
func (c *Conn) Close() error {
	var err = c.conn.Close()
	c.server.Disconnect(c.id)
	return err
}

func (c *Conn) Read() (cmd string, params string, err error) {
	message, err := c.reader.ReadString('\n')
	if err != nil {
		return
	}
	message = strings.TrimSpace(message)
	var splits = strings.SplitN(message, " ", 2)
	cmd = strings.ToUpper(splits[0])
	if len(splits) > 1 {
		params = splits[1]
	}
	return
}

// Send отправляет сообщение клиенту.
func (c *Conn) Send(command string, params ...string) error {
	var buf bytes.Buffer
	buf.WriteString(command)
	for _, param := range params {
		buf.WriteRune(' ')
		buf.WriteString(param)
	}
	buf.WriteRune('\n')
	_, err := buf.WriteTo(c.conn)
	return err
}

// Info возвращает строковое представление информации о соединении.
func (c *Conn) Info() string {
	var result = fmt.Sprintf("%s %s %s", c.id, c.addr, c.status)
	return result
}

// SetStatus устанавливает новое статусное сообщение клиента.
func (c *Conn) SetStatus(status string) {
	c.status = status
}

// serve обрабатывает команды, присылаемые клиентом.
func (c *Conn) serve() {
	for {
		var cmd, params, err = c.Read()
		if err != nil {
			c.Close()
			return
		}
		switch cmd {
		case CONNECT:
			c.Send(ERROR, "Already connected")
		case DISCONNECT:
			c.Send(OK, "Disconnected")
			c.Close()
			return
		case STATUS:
			c.SetStatus(params)
			c.Send(OK, "Status is set")
		case INFO:
			c.Send(INFO, c.server.Info(params))
		case PING:
			c.Send(PONG, params)
		case PONG, ERROR, OK, SHUTDOWN:
			c.Send(OK, "Ignored")
		default:
			c.Send(ERROR, "Unknown command", cmd)
		}
	}
}
