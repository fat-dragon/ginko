package config

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"fat-dragon.org/ginko/ftn"
	"fat-dragon.org/ginko/spool"
	"github.com/yosuke-furukawa/json5/encoding/json5"
)

// Config represents the system's configuration.
type Config struct {
	Admin    string                `json:"admin"`
	System   string                `json:"system"`
	Location string                `json:"location"`
	Nets     []Net                 `json:"nets"`
	Links    map[ftn.Address]*Link `json:"-"`
}

// Net represents a configured network this node has joined.
type Net struct {
	Name     string       `json:"name"`
	Address  ftn.Address  `json:"address"`
	Links    []Link       `json:"links"`
}

type Link struct {
	Address   ftn.Address  `json:"address"`
	Password  string       `json:"password"`
	InSpool   spool.Spool  `json:"in"`
	OutSpool  spool.Spool  `json:"out"`
	PollTime  PollInterval `json:"poll"`
	LinkedNet *Net         `json:"-"`
}

type PollInterval time.Duration

// Unmarshal a time.Duration from a string in a JSON stream.
// We parse the duration from the string and return that.
func (d *PollInterval) UnmarshalJSON(data []byte) error {
	var durationStr string
	if err := json5.Unmarshal(data, &durationStr); err != nil {
		return err
	}
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return err
	}
	*d = PollInterval(duration)
	return nil
}

func (c Config) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "System:   %q\n", c.System)
	fmt.Fprintf(&b, "Admin:    %q\n", c.Admin)
	fmt.Fprintf(&b, "Location: %q\n", c.Location)
	return b.String()
}

func (c *Config) Addresses() []ftn.Address {
	addresses := make([]ftn.Address, len(c.Nets))
	for i, net := range c.Nets {
		addresses[i] = net.Address
	}
	return addresses
}

// ParseFromString parses the JSON5 representation of the
// system configuration from a string into a Config struct.
func ParseFromString(data string) (*Config, error) {
	var c Config
	err := json5.Unmarshal([]byte(data), &c)
	if err != nil {
		return nil, fmt.Errorf("Error parsing configuration: %v", err)
	}
	c.Links = make(map[ftn.Address]*Link)
	for i := range c.Nets {
		for j := range c.Nets[i].Links {
			c.Nets[i].Links[j].LinkedNet = &c.Nets[i]
			c.Links[c.Nets[i].Links[j].Address] = &c.Nets[i].Links[j]
		}
	}
	return &c, nil
}

// ParseFile parses the system configuration as JSON5 text
// from the given file, and returns the parsed Config struct.
func ParseFile(file string) (*Config, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Error reading configuration file: %v", err)
	}
	return ParseFromString(string(data))
}
