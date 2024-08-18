package main

import (
	"github.com/gaurav-gosain/ollamanager/manager"
	"github.com/gaurav-gosain/ollamanager/tabs"
	ollamanagerUtils "github.com/gaurav-gosain/ollamanager/utils"
)

func (cfg *gollamaConfig) ollamanager() {
	selectedTabs := []tabs.Tab{}
	if cfg.Install {
		selectedTabs = append(selectedTabs, tabs.INSTALL)
	}
	if cfg.Manage {
		selectedTabs = append(selectedTabs, tabs.MANAGE)
	}
	if cfg.Monitor {
		selectedTabs = append(selectedTabs, tabs.MONITOR)
	}
	approvedActions := []tabs.ManageAction{
		tabs.UPDATE,
		tabs.DELETE,
	}

	result, err := manager.Run(selectedTabs, approvedActions)

	err = ollamanagerUtils.PrintActionResult(
		result,
		err,
	)
}
