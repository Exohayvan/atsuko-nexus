// Package main is the entry point for the Atsuko Nexus application.
// It initializes logging, node identification, starts a periodic updater, and launches the user interface.
package main

import (
	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/nodeid"
	"atsuko-nexus/src/ui"
	"atsuko-nexus/src/updater"
	"atsuko-nexus/src/p2p"
	"time"
)

// main initializes the node and begins execution.
// It starts the updater in a separate goroutine to run every 10 minutes while the main thread runs the interactive user interface.
func main() {
	// Generate a unique Node ID based on system-specific data
	nodeID := nodeid.GetNodeID()

	// Log the startup event with the generated Node ID
	logger.Log("INFO", "MAIN", "Script started with ID: "+nodeID)

	// Start Bootstrap Listener
	p2p.StartNexusListener()

	// Start Bootstrap
	p2p.Bootstrap()

	// Start the updater in a background goroutine to run every 5 minutes
	go func() {
		for {
			logger.Log("DEBUG", "UPDATER", "Running updater check")
			updater.RunUpdater()
			time.Sleep(5 * time.Minute)
		}
	}()
	go func() {
		for {
			logger.Log("DEBUG", "TAPSYNC", "Running TapSync")
			p2p.TapSync()
			time.Sleep(1 * time.Minute)
		}
	}()
	// Start the terminal user interface.
	// This call blocks the main thread until the UI exits.
	ui.Start(nodeID)
}
