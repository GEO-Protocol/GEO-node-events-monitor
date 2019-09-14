package conf

import (
	"encoding/json"
	"errors"
	"io/ioutil"
)

type HandlerSettings struct {
	NodeDirPath string `json:"node_path"`
}

type ServiceSettings struct {
	SendEvents bool   `json:"allow_send_events"`
	EventsFile string `json:"events_file_name"`
	URL        string `json:"url"`
	SendLogs   bool   `json:"allow_send_logs"`
}

type Settings struct {
	Handler HandlerSettings `json:"handler"`
	Service ServiceSettings `json:"collecting_data_service"`
}

var (
	Params = Settings{}
)

func LoadSettings() error {
	// Reading configuration file
	bytes, err := ioutil.ReadFile("conf.json")
	if err != nil {
		return errors.New("Can't read configuration. Details: " + err.Error())
	}

	err = json.Unmarshal(bytes, &Params)
	if err != nil {
		return errors.New("Can't read configuration. Details: " + err.Error())
	}

	return nil
}
