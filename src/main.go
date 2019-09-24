package main

import (
	"conf"
	"handler"
	"logger"
	"os"
)

func main() {
	err := conf.LoadSettings()
	if err != nil {
		println("ERROR: Settings can't be loaded." + err.Error())
		os.Exit(1)
	}

	err = logger.Init()
	if err != nil {
		logger.Error("Can't init logger.")
		os.Exit(-1)
	}

	// todo : check if node runs
	// todo : check if another events monitor doesn't

	if conf.Params.Service.SendEvents {
		ioDirPath := conf.Params.Handler.NodeDirPath

		_, err = os.Stat(ioDirPath)
		if err != nil {
			logger.Error("Can't find node, there is no node folder " + err.Error())
			os.Exit(-1)
		}

		node := handler.NewNode()
		err := node.AttachEventsMonitor()
		if err != nil {
			logger.Error("Can't attach to the node " + err.Error())
			os.Exit(-1)
		}
	}

	if conf.Params.Service.SendLogs {
		go handler.UploadNodeLogFileTask()
	}

	logger.Info("Handler started")

	select {}
}
