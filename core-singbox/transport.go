//go:build !cgo && !ios

package main

// Desktop entry: connect back to the app's IPC endpoint (named pipe on
// Windows, unix socket / TCP on others) and serve actions.

import (
	"io"
)

func startServer(arg string) {
	var err error
	conn, err = dial(arg)
	if err != nil {
		panic(err.Error())
	}

	defer func(conn io.Closer) {
		_ = conn.Close()
	}(conn)

	serveConn(conn)
}
