package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
)

// Ticket struct used in DB and websocket messages
type Ticket struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Phone       string    `json:"phone"`
	Room        string    `json:"room"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

var db *sql.DB

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // ubah untuk produksi
	},
}

// broadcaster: manages admin websocket connections and broadcasting messages
type Broadcaster struct {
	mu    sync.Mutex
	conns map[*websocket.Conn]bool
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{conns: make(map[*websocket.Conn]bool)}
}

func (b *Broadcaster) Add(c *websocket.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.conns[c] = true
}

func (b *Broadcaster) Remove(c *websocket.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.conns, c)
}

func (b *Broadcaster) Broadcast(event string, payload interface{}) {
	msg := map[string]interface{}{"event": event, "payload": payload}
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.conns {
		c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.WriteJSON(msg); err != nil {
			log.Printf("ws write error: %v, removing connection", err)
			c.Close()
			delete(b.conns, c)
		}
	}
}

var broad = NewBroadcaster()

func main() {
	// flags for config
	addr := flag.String("addr", ":8080", "http service address")
	dsn := flag.String("dsn", "root:password@tcp(127.0.0.1:3306)/ticketing_db?parseTime=true", "MySQL DSN")
	staticDir := flag.String("static", "../static", "static files dir")
	flag.Parse()

	var err error
	db, err = sql.Open("mysql", *dsn)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	mux := http.NewServeMux()
	// serve static files (index.html, admin.html, styles.css)
	mux.Handle("/", http.FileServer(http.Dir(*staticDir)))
	mux.HandleFunc("/api/tickets", ticketsHandler)      // GET, POST
	mux.HandleFunc("/api/tickets/", ticketItemHandler) // GET, PUT, DELETE
	mux.HandleFunc("/ws/admin", adminWsHandler)        // websocket for admins

	log.Printf("Server starting on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// ticketsHandler supports GET (list) and POST (create)
func ticketsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query("SELECT id, name, phone, room, description, status, priority, created_at, updated_at FROM tickets ORDER BY created_at DESC")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var res []Ticket
		for rows.Next() {
			var t Ticket
			if err := rows.Scan(&t.ID, &t.Name, &t.Phone, &t.Room, &t.Description, &t.Status, &t.Priority, &t.CreatedAt, &t.UpdatedAt); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			res = append(res, t)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)

	case http.MethodPost:
		var t Ticket
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		q := `INSERT INTO tickets (name, phone, room, description, status, priority) VALUES (?, ?, ?, ?, ?, ?)`
		res, err := db.Exec(q, t.Name, t.Phone, t.Room, t.Description, t.Status, t.Priority)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		id, _ := res.LastInsertId()
		t.ID = int(id)
		// read created_at / updated_at
		_ = db.QueryRow("SELECT created_at, updated_at FROM tickets WHERE id = ?", id).Scan(&t.CreatedAt, &t.UpdatedAt)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(t)

		// broadcast new ticket to admin websockets
		broad.Broadcast("ticket_created", t)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ticketItemHandler supports GET /:id, PUT /:id, DELETE /:id
func ticketItemHandler(w http.ResponseWriter, r *http.Request) {
	// simple path parsing: /api/tickets/{id}
	var id int
	_, err := fmt.Sscanf(r.URL.Path, "/api/tickets/%d", &id)
	if err != nil || id == 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		var t Ticket
		if err := db.QueryRow("SELECT id, name, phone, room, description, status, priority, created_at, updated_at FROM tickets WHERE id = ?", id).
			Scan(&t.ID, &t.Name, &t.Phone, &t.Room, &t.Description, &t.Status, &t.Priority, &t.CreatedAt, &t.UpdatedAt); err != nil {
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(t)

	case http.MethodPut:
		var t Ticket
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		q := `UPDATE tickets SET name=?, phone=?, room=?, description=?, status=?, priority=? WHERE id=?`
		_, err := db.Exec(q, t.Name, t.Phone, t.Room, t.Description, t.Status, t.Priority, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// fetch updated row
		if err := db.QueryRow("SELECT id, name, phone, room, description, status, priority, created_at, updated_at FROM tickets WHERE id = ?", id).
			Scan(&t.ID, &t.Name, &t.Phone, &t.Room, &t.Description, &t.Status, &t.Priority, &t.CreatedAt, &t.UpdatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(t)
		broad.Broadcast("ticket_updated", t)

	case http.MethodDelete:
		_, err := db.Exec("DELETE FROM tickets WHERE id = ?", id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		broad.Broadcast("ticket_deleted", map[string]int{"id": id})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// adminWsHandler upgrades connection and keeps it open. Admin clients receive broadcasts
func adminWsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}
	defer c.Close()
	broad.Add(c)
	// send current ticket list immediately
	rows, err := db.Query("SELECT id, name, phone, room, description, status, priority, created_at, updated_at FROM tickets ORDER BY created_at DESC")
	if err == nil {
		var res []Ticket
		for rows.Next() {
			var t Ticket
			_ = rows.Scan(&t.ID, &t.Name, &t.Phone, &t.Room, &t.Description, &t.Status, &t.Priority, &t.CreatedAt, &t.UpdatedAt)
			res = append(res, t)
		}
		_ = c.WriteJSON(map[string]interface{}{"event": "init", "payload": res})
	}

	// keep reading to detect closed connection
	for {
		var msg map[string]interface{}
		if err := c.ReadJSON(&msg); err != nil {
			break
		}
	}
	broad.Remove(c)
}
