package main

import (
	"log"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}
var store = sessions.NewCookieStore([]byte("secret"))
var clients = make(map[*websocket.Conn]bool) //active connections
var broadcast = make(chan map[string]string) //channel for broadcast

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "login.html")
	})

	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/chat", handleChatPage)
	http.HandleFunc("/ws", handleConnections)
	go handleMessages()

	log.Println("Server started !!")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
func handleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	secretCode := r.FormValue("secretCode")
	if username != "" && secretCode == "0129384756" {
		session, _ := store.Get(r, "session")
		session.Values["username"] = username
		session.Save(r, w)
		http.Redirect(w, r, "/chat", http.StatusFound)
	} else {
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func handleChatPage(w http.ResponseWriter, r *http.Request) {

	session, _ := store.Get(r, "session")
	if session.Values["username"] == nil {
		http.Redirect(w, r, "/", http.StatusFound)
	}
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
		log.Fatal(err)
	}
	defer ws.Close()
	clients[ws] = true

	for {
		var msg map[string]string
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Println("Read error: ", err)
			delete(clients, ws)
			break
		}

		broadcast <- msg
	}
}
