// Package main is the entry point for the Atsuko Nexus application.
// It initializes logging, node identification, starts a periodic updater, and launches the user interface.
package main

import (
	"atsuko-nexus/src/logger"  // Custom logger for formatted terminal output
	"atsuko-nexus/src/nodeid"  // Node ID generation based on system info
	"atsuko-nexus/src/ui"      // TUI (Text-based User Interface) using Bubble Tea
	"atsuko-nexus/src/updater" // Auto-updater that fetches latest release from GitHub
	"time"
)

// main initializes the node and begins execution.
// It starts the updater in a separate goroutine to run every 10 minutes while the main thread runs the interactive user interface.
func main() {
	// Generate a unique Node ID based on system-specific data
	nodeID := nodeid.GetNodeID()

	// Log the startup event with the generated Node ID
	logger.Log("INFO", "MAIN", "Script started with ID: "+nodeID)

	// Start the updater in a background goroutine to run every 10 minutes
	go func() {
		for {
			// Log that the updater is running
			logger.Log("DEBUG", "UPDATER", "Running updater check")

			// Execute the updater logic to check and apply new releases if needed
			updater.RunUpdater()

			// Wait 10 minutes before checking again
			time.Sleep(10 * time.Minute)
		}
	}()

	// Start the terminal user interface.
	// This call blocks the main thread until the UI exits.
	ui.Start(nodeID)
}
