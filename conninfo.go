package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ConnInfo описывает информацию о соединении.
type ConnInfo struct {
	id      string
	addr    string    // IP-адрес и порт соединения
	addr2   string    // передающийся адрес и порт
	updated time.Time // дата и время последнего обновления информации
	status  string    // строка со статусом
	conn    net.Conn  // сокетное соединение
}

// NewConnInfo возвращает новую информацию о соединении.
func NewConnInfo(conn net.Conn, id, addr, addr2 string) *ConnInfo {
	if addr2 == "" {
		addr2 = "0.0.0.0:0"
	}
	var ci = &ConnInfo{
		id:      id,
		addr:    addr,
		addr2:   addr2,
		conn:    conn,
		updated: time.Now().UTC(),
	}
	return ci
}

// Close закрывает сокетное соединение.
func (ci *ConnInfo) Close() error {
	if ci.conn != nil {
		return ci.conn.Close()
	}
	return nil
}

// String возвращает строковое представление информации о соединении.
func (ci *ConnInfo) String() string {
	return fmt.Sprintf("%s %s %s %s", ci.addr, ci.addr2, ci.updated.UTC().Format(time.RFC3339), ci.status)
}

// SetStatus задает новый текст статуса.
func (ci *ConnInfo) SetStatus(status string) {
	ci.status = status
	ci.Update()
}

// Update устанавливает в текущее время последнего обновления информации.
func (ci *ConnInfo) Update() {
	ci.updated = time.Now().UTC()
}

// List описывает список с информацией о соединениях.
type List struct {
	connections map[string]*ConnInfo // информация о всех соединениях
	mu          sync.RWMutex
}

// NewList инициализирует и возвращает новый список с информацией о соединениях.
func NewList() *List {
	return &List{
		connections: make(map[string]*ConnInfo),
	}
}

// Add добавляет новую информацию о соединении.
func (l *List) Add(conn net.Conn, id, addr, addr2 string) {
	l.mu.Lock()
	if info, ok := l.connections[id]; ok {
		info.Close()
	}
	l.connections[id] = NewConnInfo(conn, id, addr, addr2)
	l.mu.Unlock()
}

// SetStatus изменяет статусное сообщение соединения, если оно зарегистрировано с таким идентификатором.
func (l *List) SetStatus(id, status string) {
	l.mu.RLock()
	if ci, ok := l.connections[id]; ok {
		ci.SetStatus(status)
	}
	l.mu.RUnlock()
}

// Update обновляет время актуальности данных, устанавливая его в текущее, если информация о
// соединении с таким идентификатором зарегистрирована.
func (l *List) Update(id string) {
	l.mu.RLock()
	if ci, ok := l.connections[id]; ok {
		ci.Update()
	}
	l.mu.RUnlock()
}

// Remove удаляет информацию о соединении с указанным идентификатором.
func (l *List) Remove(id string) {
	l.mu.Lock()
	if info, ok := l.connections[id]; ok {
		info.Close()
		delete(l.connections, id)
	}
	l.mu.Unlock()
}

// Info возвращает информацию о соединении.
func (l *List) Info(id string) *ConnInfo {
	l.mu.RLock()
	var ci = l.connections[id]
	l.mu.RUnlock()
	return ci
}

func (l *List) List() string {
	l.mu.RLock()
	var result = make([]string, 0, len(l.connections))
	for id := range l.connections {
		result = append(result, id)
	}
	l.mu.RUnlock()
	return strings.Join(result, ", ")
}
