package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	CONFIGURATION_FILE = "configuration.json"

	HEALTH_CHECK_INTERVAL = 30 //Sec
	MIN_MINERS            = 1
	FETCH_TIMEOUT         = 10 //SEC
)

var (
	Config Configuration
)

type Configuration struct {
	ThisIpport string
	This       struct {
		Ip   string `json:"ip"`
		Port string `json:"port"`
	} `json:"multisig"`
	BootstrapIpport string
	Bootstrap       struct {
		Ip   string `json:"ip"`
		Port string `json:"port"`
	} `json:"bootstrap"`
	ClientIpport string
	Client       struct {
		Ip   string `json:"ip"`
		Port string `json:"port"`
	} `json:"client"`
}

func LoadConfiguration() (config Configuration) {
	configFile, err := os.Open(CONFIGURATION_FILE)
	defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	config.ThisIpport = config.This.Ip + ":" + config.This.Port
	config.BootstrapIpport = config.Bootstrap.Ip + ":" + config.Bootstrap.Port
	config.ClientIpport = config.Client.Ip + ":" + config.Client.Port

	return config
}
