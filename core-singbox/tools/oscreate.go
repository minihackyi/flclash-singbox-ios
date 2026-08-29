//go:build ignore

package main

import "os"

func osCreate(name string) *os.File {
	f, _ := os.Create(name)
	return f
}
