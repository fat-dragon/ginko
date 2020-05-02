package main

import (
	"flag"
	"log"
	"net"

	"fat-dragon.org/ginko/config"
	"fat-dragon.org/ginko/proto"
)

var configFile string
var pollHost string

func init() {
	const defaultConfigFile = "/opt/local/etc/ginko.conf"
	flag.StringVar(&configFile, "c", defaultConfigFile, "config file name")
	flag.StringVar(&pollHost, "p", "", "Host to poll")
}

func main() {
	flag.Parse()
	config, err := config.ParseFile(configFile)
	if err != nil {
		log.Fatalf("cannot read config file: %v", err)
	}
	if pollHost != "" {
		poll(config, pollHost)
	} else {
		server(config)
	}
}

func server(config *config.Config) {
	server, err := net.Listen("tcp", ":24554")
	if err != nil {
		log.Fatalf("cannot listen: %v", err)
	}
	defer server.Close()
	for {
		client, err := server.Accept()
		if err != nil {
			log.Fatalf("accept failed: %v", err)
		}
		go proto.Receiver(config, client)
	}
}

func poll(config *config.Config, host string) {
	conn, err := net.Dial("tcp", host)
	if err != nil {
		log.Fatal("poll: dial failed:", err)
	}
	defer conn.Close()
	proto.Sender(config, conn)
}
