// Package main is the process entry point for analytics-service.
// It delegates all singleton wiring and lifecycle management to lib/boot.Run().
package main

import (
	"os"

	"quickbite/analytics-service/lib/boot"
)

func main() {
	if err := boot.Run(); err != nil {
		os.Exit(1)
	}
}

