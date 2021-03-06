# IP-Locator

Сервер, позволяющий делиться информацией о своем адресе и текстовом статусе.

Для соединения с сервером используется порт `9000`. На данный момент поддерживаются следующие команды:

- `CONNECT [client_id] [+remote_address]` - сохраняет текущий адрес соединения под указанным именем. Команда может быть выполнена в любой момент, но только один раз. Повторное выполнение команды вернет ошибку. В качестве второго параметра можно указать свой адрес, который будет использоваться вместо удаленного адреса клиента.
- `STATUS [text]` - позволяет задать произвольный текст в виде статуса.
- `INFO [client_id]` - возвращает информацию о соединении с таким именем. Возвращается название, IP-адрес и порт, дата и время последнего обновления информации, текст статуса.
- `PING [text]` - возвращает ответ `PONG` с тем же текстом.
- `DISCONNECT` - закрывает текущее соединение. Информация об этом соединении будет доступна еще в течении 5 минут.
- любые другие команды не приводят к ошибке и возвращают ответ, что команда проигнорирована.
- `TO xyz\n[binary data]` - передает сообшение указанному адресату. Первые 4 байта бинарного сообщения - длинна блока включая эти четыре, то есть если блок дальше 100 байт, то длинна будет 104. Блок принимается и отдается подключенному пользователю в формате FROM zyx\n[binary data]. Размер бинарных данных может быть до 4Г


#### Примечания

- Ответ всегда в формате: `Статус` `Команда` `Дополнительная информация`.  
  Статус может быть `OK` или `ERROR`. Команда всегда повторяет переданную.
- Соединение автоматически разрывается, если в течении 5 минут не было передано ни одной команды. 
- Любая команда (даже неверная) устанавливает время обновления информации о соединении в текущее. 
- Регистр команд неважен, а вот идентификатор соединения является чувствительным к регистру. 
- Время всегда возвращается в формате RFC3339 в UTC.

#### Пример работы через telnet

	telnet localhost 9000
	Trying 127.0.0.1...
	Connected to localhost.
	Escape character is '^]'.
	CONNECT test_id 127.0.0.1:57554
	OK CONNECT test_id 127.0.0.1:57554
	STATUS text status
	OK STATUS text status
	INFO a
	ERROR INFO a not found
	INFO test_id
	OK INFO test_id 127.0.0.1:57554 127.0.0.1:57554 2015-04-10T23:30:01Z text status
	PING test ping
	OK PING test ping
	DISCONNECT
	OK DISCONNECT
	Connection closed by foreign host.

	telnet localhost 9000
	Trying 127.0.0.1...
	Connected to localhost.
	Escape character is '^]'.
	INFO test_id
	OK INFO test_id 127.0.0.1:57554 127.0.0.1:57554 2015-04-10T23:30:39Z text status
	STATUS text status
	ERROR STATUS not connected
	TEST command
	ERROR TEST unknown command
	INFO test_id
	OK INFO test_id 127.0.0.1:57554 127.0.0.1:57554 2015-04-10T23:30:39Z text status
	DISCONNECT
	OK DISCONNECT
	Connection closed by foreign host.


## Планы

- [x] Простой текстовый протокол (работа через telnet)
- [x] Сохранение информация о соединении
- [x] Опциональная установка удаленного адреса соединения
- [x] Запрос статуса соединения по имени
- [x] Установка текстового статуса и получение его в запросе
- [x] Поддержка PING/PONG для поддержки соединения
- [ ] Возможность отключения с сохранением информации о соединении в течении 5 минут (отключено, не используется)
- [x] Корректная поддержка многопоточности
- [x] Передача сообщений через сервер
- [x] Поддержка TLS
- [ ] Авторизация
