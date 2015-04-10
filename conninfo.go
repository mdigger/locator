package main

import (
	"fmt"
	"sync"
	"time"
)

// ConnInfo описывает информацию о соединении.
type ConnInfo struct {
	addr    string    // IP-адрес и порт
	updated time.Time // дата и время последнего обновления информации
	status  string    // строка со статусом
}

// NewConnInfo возвращает новую информацию о соединении.
func NewConnInfo(addr string) *ConnInfo {
	var ci = &ConnInfo{addr: addr}
	ci.Update()
	return ci
}

// String возвращает строковое представление информации о соединении.
func (ci *ConnInfo) String() string {
	return fmt.Sprintf("%s %s %s", ci.addr, ci.updated.UTC().Format(time.RFC3339), ci.status)
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
func NewList(d time.Duration) *List {
	var list = &List{connections: make(map[string]*ConnInfo)}
	// периодически очищаем информацию с устаревшими данными
	go func() {
		time.Sleep(d + d/2) // полуторный интервал задержки с очисткой
		var lastValid = time.Now().Add(-d)
		list.mu.Lock()
		for id, ci := range list.connections {
			if ci.updated.After(lastValid) {
				delete(list.connections, id)
			}
		}
		list.mu.Unlock()
	}()
	return list
}

// Add добавляет новую информацию о соединении.
func (l *List) Add(id, addr string) {
	l.mu.Lock()
	l.connections[id] = NewConnInfo(addr)
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
	delete(l.connections, id)
	l.mu.Unlock()
}

// Info возвращает информацию о соединении.
func (l *List) Info(id string) *ConnInfo {
	l.mu.RLock()
	var ci = l.connections[id]
	l.mu.RUnlock()
	return ci
}
