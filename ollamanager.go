package main

import (
	"github.com/gaurav-gosain/ollamanager/manager"
	"github.com/gaurav-gosain/ollamanager/tabs"
	ollamanagerUtils "github.com/gaurav-gosain/ollamanager/utils"
)

// Branches to the ollamanager process and exits
// Add tabs (and actions) based on the flags
func (cfg *gollamaConfig) ollamanager() {
	var selectedTabs []tabs.Tab
	var approvedActions []tabs.ManageAction
	if cfg.Install {
		selectedTabs = append(selectedTabs, tabs.INSTALL)
	}
	if cfg.Manage {
		selectedTabs = append(selectedTabs, tabs.MANAGE)
		// Approved actions for the Manage tab
		approvedActions = []tabs.ManageAction{
			tabs.UPDATE,
			tabs.DELETE,
		}
	}
	if cfg.Monitor {
		selectedTabs = append(selectedTabs, tabs.MONITOR)
	}

	// Runs ollamanager with the selected tabs and approved actions
	result, err := manager.Run(selectedTabs, approvedActions)

	// Pretty prints the result and error (if any)
	ollamanagerUtils.PrintError(
		ollamanagerUtils.PrintActionResult(
			result,
			err,
		),
	)
}
