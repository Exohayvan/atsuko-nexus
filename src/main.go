package main

import (
	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/nodeid"
	"atsuko-nexus/src/ui"
	"atsuko-nexus/src/updater"
)

func main() {
	nodeID := nodeid.GetNodeID()
	logger.Log("INFO", "MAIN", "Script started with ID: "+nodeID)

	logger.Log("DEBUG", "MAIN", "Calling Updater.go")
	updater.RunUpdater()
	ui.Start(nodeID)
}
