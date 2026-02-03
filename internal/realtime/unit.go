package realtime

import "github.com/gofiber/websocket/v2"

type UnitsHub struct {
	Register   chan *websocket.Conn
	Unregister chan *websocket.Conn
	Broadcast  chan []byte
	Clients    map[*websocket.Conn]bool
}

var Units = UnitsHub{
	Register:   make(chan *websocket.Conn),
	Unregister: make(chan *websocket.Conn),
	Broadcast:  make(chan []byte),
	Clients:    make(map[*websocket.Conn]bool),
}

func RunUnitsBroadcaster() {
	for {
		select {
		case c := <-Units.Register:
			Units.Clients[c] = true
		case c := <-Units.Unregister:
			delete(Units.Clients, c)
			c.Close()
		case msg := <-Units.Broadcast:
			for c := range Units.Clients {
				c.WriteMessage(websocket.TextMessage, msg)
			}
		}
	}
}
