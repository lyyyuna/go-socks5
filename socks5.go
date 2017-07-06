package main

import (
	"fmt"
	"net"
)

func main() {
	server := "127.0.0.1:45672"
	run(server)
}

func run(server string) {
	ln, err := net.Listen("tcp", server)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Starting server on %v", server)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Accept error: ", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	remoteaddr := conn.RemoteAddr().String()
	fmt.Println("Accepted addr: ", remoteaddr)
	defer func() {
		conn.Close()
		return
	}()
}
