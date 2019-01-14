package main

import (
	"fmt"
	"github.com/satori/go.uuid"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type mudConfig struct {
	World  worldConfig  `yaml:"world"`
	Store  storeConfig  `yaml:"store"`
	Telnet telnetConfig `yaml:"telnet"`
	WSAPI  wsAPIConfig  `yaml:"wsAPI"`
}

type worldConfig struct {
	DefaultZoneID     uuid.UUID   `yaml:"defaultZoneID"`
	DefaultLocationID uuid.UUID   `yaml:"defaultLocationID"`
	ZonesToLoad       []uuid.UUID `yaml:"zonesToLoad"`
}

type storeConfig struct {
	UseCompression    bool   `yaml:"useCompression"`
	SnapshotDirectory string `yaml:"snapshotDirectory"`
	IntentLogfile     string `yaml:"intentLogfile"`
	EventsFile        string `yaml:"eventsFile"`
}

type telnetConfig struct {
	ListenAddr string `yaml:"listenAddr"`
}

type wsAPIConfig struct {
	ListenAddr string `yaml:"listenAddr"`
}

func (mc mudConfig) SerializeToFile(filename string) error {
	fBytes, err := yaml.Marshal(mc)
	if err != nil {
		return fmt.Errorf("yaml.Marshal(): %s", err)
	}
	err = ioutil.WriteFile(filename, fBytes, 0644)
	if err != nil {
		return fmt.Errorf("ioutil.WriteFile(%q, ...): %s", filename, err)
	}
	return nil
}

func (mc *mudConfig) DeserializeFromFile(filename string) error {
	fBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("ioutil.ReadFile(%q): %s", filename, err)
	}
	err = yaml.Unmarshal(fBytes, mc)
	if err != nil {
		return fmt.Errorf("yaml.Unmarshal(): %s", err)
	}
	return nil
}
