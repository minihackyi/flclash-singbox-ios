//go:build ios

package main

// iOS entry: lives inside the Packet Tunnel Provider process (statically
// linked). Swift calls coreStartServer to spin up the action server on a
// loopback TCP port; the Flutter main process connects and speaks the same
// framed Action protocol as desktop.

import (
	"net"
	"sync"
)

var (
	listenerMu   sync.Mutex
	tcpListener  net.Listener
	listenerPort int
)

//export coreStartServer
func coreStartServer() int32 {
	listenerMu.Lock()
	defer listenerMu.Unlock()
	if tcpListener != nil {
		return int32(listenerPort)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		logError("coreStartServer listen error: %v", err)
		return 0
	}
	tcpListener = listener
	listenerPort = listener.Addr().(*net.TCPAddr).Port
	go acceptLoop(listener)
	return int32(listenerPort)
}

//export coreStopServer
func coreStopServer() {
	listenerMu.Lock()
	defer listenerMu.Unlock()
	if tcpListener != nil {
		_ = tcpListener.Close()
		tcpListener = nil
		listenerPort = 0
	}
	shutdownEngine()
}

//export coreServerPort
func coreServerPort() int32 {
	listenerMu.Lock()
	defer listenerMu.Unlock()
	return int32(listenerPort)
}

func acceptLoop(listener net.Listener) {
	for {
		client, err := listener.Accept()
		if err != nil {
			listenerMu.Lock()
			closed := tcpListener == nil
			listenerMu.Unlock()
			if closed {
				return
			}
			logError("accept error: %v", err)
			continue
		}
		go handleClient(client)
	}
}

func handleClient(client net.Conn) {
	connMu.Lock()
	conn = client
	connMu.Unlock()
	serveConn(client)
	connMu.Lock()
	if iface, ok := conn.(net.Conn); ok && iface == client {
		conn = nil
	}
	connMu.Unlock()
}
