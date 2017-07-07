package main

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
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

	buf1 := readBytes(conn, 2)
	protocolCheck(buf1[0] == 0x05)
	nmethods := int(buf1[1])
	readBytes(conn, nmethods)

	// no auth required
	conn.Write([]byte{0x05, 0x00})

	// handshake finished
	// get request
	buf3 := readBytes(conn, 4)
	protocolCheck(buf3[0] == 0x05)
	protocolCheck(buf3[2] == 0x00)

	cmd := buf3[1]
	if cmd != 0x01 {
		// 0x07: command not support
		conn.Write(errReply(0x07))
		return
	}

	atyp := buf3[3]
	if atyp != 0x01 && atyp != 0x03 {
		conn.Write(errReply(0x08))
		return
	}

	var dstAddr string
	if atyp == 0x01 {
		// IPv4
		buf4 := readBytes(conn, 6)
		dstAddr = fmt.Sprintf("%d.%d.%d.%d:%d", buf4[0], buf4[1],
			buf4[2], buf4[3], int(buf4[4])*256+int(buf4[5]))
	} else {
		// FQDN
		buf4 := readBytes(conn, 1)
		nmlen := int(buf4[0])            // domain length
		buf5 := readBytes(conn, nmlen+2) //domain + port
		dstAddr = fmt.Sprintf("%s:%d", buf5[0:nmlen],
			int(buf5[nmlen])*256+int(buf5[nmlen+1]))
	}

	connectRemoteServer(dstAddr, conn)
}

func connectRemoteServer(dstAddr string, conn net.Conn) {
	fmt.Println("Trying to connect to remote server: %s ...", dstAddr)

	dstconn, err := net.Dial("tcp", dstAddr)
	if err != nil {
		fmt.Println("Failed to connect to %s: %s", dstAddr, err)
		conn.Write(errReply(0x05))
		return
	}

	defer func() {
		dstconn.Close()
		fmt.Println("Remote disconnect", dstAddr)
	}()

	// reply to connect cmd
	buf := make([]byte, 10)
	copy(buf, []byte{0x05, 0x00, 0x00, 0x01})
	packNetAddr(dstconn.RemoteAddr(), buf[4:])
	conn.Write(buf)

	// pipe the two connection
	shutdown := make(chan bool, 2)
	go pipeConn(conn, dstconn, shutdown)
	go pipeConn(dstconn, conn, shutdown)

	<-shutdown
}

func pipeConn(src io.Reader, dst io.Writer, shutdown chan bool) {
	defer func() {
		shutdown <- true
	}()

	buf := make([]byte, 4096)
	for {
		n, err := src.Read(buf)
		if err != nil {
			if !(err == io.EOF || isUseOfClosedConn(err)) {
				fmt.Println("error reading %s: %s\n", src, err)
			}
			break
		}
		fmt.Println(string(buf[:20]))
		_, err = dst.Write(buf[:n])
		if err != nil {
			fmt.Println("error writing %s: %s\n", src, err)
			break
		}

	}
}

func readBytes(conn io.Reader, count int) (buf []byte) {
	buf = make([]byte, count)
	if _, err := io.ReadFull(conn, buf); err != nil {
		panic(err)
	}
	return
}

func protocolCheck(assert bool) {
	if !assert {
		panic("Protocol err.")
	}
}

func errReply(reason byte) []byte {
	return []byte{0x05, reason, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

func packNetAddr(addr net.Addr, buf []byte) {
	ipport := addr.String()
	pair := strings.Split(ipport, ":")
	ipstr, portstr := pair[0], pair[1]
	port, err := strconv.Atoi(portstr)
	if err != nil {
		panic(fmt.Sprintf("invalid address %s", ipport))
	}

	copy(buf[:4], net.ParseIP(ipstr).To4())
	buf[4] = byte(port / 256)
	buf[5] = byte(port % 256)
}

func isUseOfClosedConn(err error) bool {
	operr, ok := err.(*net.OpError)
	return ok && operr.Err.Error() == "use of closed network connection"
}
