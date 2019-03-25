package conf

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"strconv"
)

type HandlerSettings struct {
	NodeDirPath              string `json:"node_path"`
}

type ServiceSettings struct {
	SendEvents	bool	`json:"allow_send_events"`
	SendLogs	bool	`json:"allow_send_logs"`
	Host 		string	`json:"host"`
	Port 		uint16	`json:"port"`
}

type Settings struct {
	Handler 		HandlerSettings `json:"handler"`
	Service			ServiceSettings	`json:"collecting_data_service"`
}

func (s ServiceSettings) ServiceInterface() string {
	return "http://" + s.Host + ":" + strconv.Itoa(int(s.Port))
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
