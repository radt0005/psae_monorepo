// Package main is a thin shim that runs the spade-scheduler binary from
// cmd/spade-scheduler.  Keeping the top-level main.go means `go run .`
// from the server directory continues to work.
package main

import "spade_server/cmd/spade-scheduler/app"

func main() {
	app.Main()
}
