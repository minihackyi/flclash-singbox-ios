//go:build ignore

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

func main() {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("223.5.5.5"), Port: 53})
	if err != nil {
		fmt.Println("dial:", err)
		return
	}
	defer conn.Close()
	query := make([]byte, 18)
	binary.BigEndian.PutUint16(query[0:], 0x1234)
	query[2] = 0x01
	query[5] = 1
	// qname www.qq.com
	name := []byte{3, 'w', 'w', 'w', 2, 'q', 'q', 3, 'c', 'o', 'm', 0}
	q := append(query, name...)
	q = append(q, 0, 1, 0, 1)
	query = q
	binary.BigEndian.PutUint16(query[0:], 0x1234)
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(query); err != nil {
		fmt.Println("write:", err)
		return
	}
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("read:", err)
		return
	}
	fmt.Printf("raw udp dns OK, %d bytes\n", n)
}
