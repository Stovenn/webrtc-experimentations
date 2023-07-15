package main

import (
	"net/http"
	"webrtc_media_server/ws"

	"github.com/gorilla/mux"
)

func main() {

	r := mux.NewRouter()
	r.HandleFunc("/ws", ws.HandleWebsocket)

	http.ListenAndServe(":8080", r)
}
