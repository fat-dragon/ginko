package sender

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"fat-dragon.org/ginko/config"
	"fat-dragon.org/ginko/frame"
	"fat-dragon.org/ginko/proto/auth"
	"fat-dragon.org/ginko/proto/session"
	"fat-dragon.org/ginko/proto/transfer"
)

func Run(ctx context.Context, config *config.Config, c net.Conn) error {
	s := session.NewSession(ctx, config, c)
	return s.Run(ctx, start)
}

func start(ctx context.Context, s *session.Session) (session.State, error) {
	if !s.WriteSyncFrames(ctx,
		frame.NewNull("SYS "+s.Config.System),
		frame.NewNull("ZYZ "+s.Config.Admin),
		frame.NewNull("LOC "+s.Config.Location),
		frame.NewNull("VER ginko/0.0.1/OpenBSD/x86_64 binkp/1.0"),
		frame.NewNull("TIME "+time.Now().Format(time.RFC1123Z)),
		frame.NewAddress(s.Config.Addresses()...)) {
		return session.End(ctx, s, errors.New("Error sending initial frames"))
	}
	return senderWaitForAddress, nil
}

func senderWaitForAddress(ctx context.Context, s *session.Session) (session.State, error) {
	select {
	case <-ctx.Done():
		return session.End(ctx, s, nil)
	case f, ok := <-s.ReadFrames:
		if ctx.Err() != nil || f == nil || !ok {
			err := errors.New("Error waiting for address")
			log.Println(err)
			return session.End(ctx, s, err)
		}
		switch frame := f.(type) {
		case *frame.AddressCmd:
			log.Println("received:", frame)
			if s.LinkAddresses(frame.Addresses()) == nil {
				err := errors.New("Unlinked session")
				log.Println(err)
				return session.End(ctx, s, err)
			}
			return sendResponse, nil
		case *frame.ErrorCmd:
			err := fmt.Errorf("received: %v", frame)
			log.Println(err)
			return session.End(ctx, s, err)
		case *frame.BusyCmd:
			err := fmt.Errorf("received: %v", frame)
			log.Println(err)
			return session.End(ctx, s, err)
		case *frame.NullCmd:
			log.Println("received:", frame)
		case *frame.OptCmd:
			log.Println("received:", frame)
			for _, text := range frame.Options() {
				if strings.HasPrefix(text, "CRAM-") {
					return saveChallenge(ctx, text, s)
				}
			}
		default:
			err := fmt.Errorf("unexpected frame: %v", frame)
			log.Println(err)
			s.SendErrorCmd(ctx, "Unexpected frame received")
			return session.End(ctx, s, err)
		}
		return senderWaitForAddress, nil
	}
}

func saveChallenge(ctx context.Context, text string, s *session.Session) (session.State, error) {
	fields := strings.Split(text, "-")
	if len(fields) != 3 {
		err := fmt.Errorf("Malformed challenge: %q", text)
		log.Println(err)
		s.SendErrorCmd(ctx, "Malformed challenge")
		return session.End(ctx, s, err)
	}
	s.HashStr = fields[1]
	challenge, err := auth.DecodeChallenge(fields[2])
	if err != nil {
		err := fmt.Errorf("Failed to decode challenge %q: %v", text, err)
		log.Println(err)
		s.SendErrorCmd(ctx, "Challenge decode failed")
		return session.End(ctx, s, err)
	}
	s.Challenge = challenge
	return senderWaitForAddress, nil
}

func sendResponse(ctx context.Context, s *session.Session) (session.State, error) {
	response, err := auth.GenerateResponse(s.HashStr, s.Challenge, s.Link.Password)
	if err != nil {
		err := fmt.Errorf("Failed to generate response: %v", err)
		log.Println(err)
		s.SendErrorCmd(ctx, "Challenge response generation failed")
		return session.End(ctx, s, err)
	}
	if !s.WriteSyncFrame(ctx, frame.NewPassword("CRAM-"+s.HashStr+"-"+response)) {
		err := errors.New("Failed to write PWD")
		log.Println(err)
		return session.End(ctx, s, err)
	}
	return waitForOk, nil
}

func waitForOk(ctx context.Context, s *session.Session) (session.State, error) {
	select {
	case <-ctx.Done():
		return session.End(ctx, s, nil)
	case f, ok := <-s.ReadFrames:
		if ctx.Err() != nil || f == nil || !ok {
			err := errors.New("Error waiting for challenge")
			log.Println(err)
			return session.End(ctx, s, err)
		}
		switch frame := f.(type) {
		case *frame.OkCmd:
			log.Println("received:", frame)
			return transfer.Start, nil
		case *frame.ErrorCmd:
			err := fmt.Errorf("received: %v", frame)
			log.Println(err)
			return session.End(ctx, s, err)
		case *frame.BusyCmd:
			err := fmt.Errorf("received: %v", frame)
			log.Println(err)
			return session.End(ctx, s, err)
		case *frame.NullCmd:
			log.Println("received:", frame)
		case *frame.OptCmd:
			log.Println("received:", frame)
		default:
			err := fmt.Errorf("unexpected frame: %v", frame)
			log.Println(err)
			s.SendErrorCmd(ctx, "Unexpected frame received")
			return session.End(ctx, s, err)
		}
		return waitForOk, nil
	}
}
