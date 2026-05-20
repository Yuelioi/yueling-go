package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func try(label, url string, header http.Header) {
	fmt.Printf("\n[%s] %s\n", label, url)
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, resp, err := dialer.Dial(url, header)
	if err != nil {
		if resp != nil {
			fmt.Printf("  FAILED  HTTP %s\n", resp.Status)
		} else {
			fmt.Printf("  FAILED  %v\n", err)
		}
		return
	}
	defer conn.Close()
	fmt.Println("  OK  connected")
	conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		fmt.Printf("  read: %v\n", err)
		return
	}
	fmt.Printf("  msg: %s\n", msg)
}

func main() {
	base := "ws://192.168.5.7:9077/onebot/v11/ws"
	token := "UBUE0N[kni35^xL("

	// 1. header Bearer
	try("header Bearer", base, http.Header{"Authorization": {"Bearer " + token}})

	// 2. query param
	try("query param", base+"?access_token="+token, nil)

	// 3. no token
	try("no token", base, nil)
}
