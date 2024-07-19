package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}
var store = sessions.NewCookieStore([]byte("secret"))
var clients = make(map[*websocket.Conn]bool) // active connections
var broadcast = make(chan map[string]string) // channel for broadcast

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "login.html")
	}).Methods("GET")

	router.HandleFunc("/login", handleLogin).Methods("POST")
	router.HandleFunc("/chat", checkSession(handleChatPage)).Methods("GET")
	router.HandleFunc("/ws", handleConnections)

	go handleMessages()

	server := &http.Server{
		Addr: ":8080",
	}

	go func() {
		log.Println("Server started on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}

func checkSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "session")
		if err != nil || session.Values["username"] == nil {
			log.Println("Unauthorized access attempt to:", r.URL.Path)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		next(w, r)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	secretCode := r.FormValue("secretCode")
	if username != "" && secretCode == "999999" {
		session, err := store.Get(r, "session")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		session.Values["username"] = username
		if err := session.Save(r, w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/chat", http.StatusFound)
	} else {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
	}
}

func handleChatPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "chat.html")
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer ws.Close()
	clients[ws] = true

	for {
		var msg map[string]string
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Println("Read error:", err)
			delete(clients, ws)
			break
		}
		broadcast <- msg
	}
}
