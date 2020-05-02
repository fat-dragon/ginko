package proto

import (
	"context"
	"log"
	"net"

	"fat-dragon.org/ginko/config"
	"fat-dragon.org/ginko/proto/receiver"
	"fat-dragon.org/ginko/proto/sender"
)

// Receiver runs a session for an incoming connection.
func Receiver(config *config.Config, conn net.Conn) {
	defer conn.Close()
	log.Println("Receiver session starting")
	err := receiver.Run(context.Background(), config, conn)
	if err != nil {
		log.Println("Receiver session failed:", err)
		return
	}
	log.Println("Receiver session successful")
}

// Sender runs a session for an outgoing connection.
func Sender(config *config.Config, conn net.Conn) {
	defer conn.Close()
	log.Println("Sender session starting")
	err := sender.Run(context.Background(), config, conn)
	if err != nil {
		log.Println("Sender session failed:", err)
		return
	}
	log.Println("Sender session sucessful")
}
