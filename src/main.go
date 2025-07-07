package main

import (
	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/nodeid"
	"atsuko-nexus/src/ui"
)

func main() {
	nodeID := nodeid.GetNodeID()
	logger.Log("INFO", "MAIN", "Script started with ID: "+nodeID)

	ui.Start(nodeID)
}
