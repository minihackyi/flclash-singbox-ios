//go:build ios

package main

// The c-archive build requires the main package to declare main even though
// the archive itself is consumed by Swift (coreStartServer is the entry).
func main() {}
