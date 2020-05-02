package receiver

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

func Run(ctx context.Context, config *config.Config, conn net.Conn) error {
	s := session.NewSession(ctx, config, conn)
	return s.Run(ctx, start)
}

func start(ctx context.Context, s *session.Session) (session.State, error) {
	s.Challenge = auth.GenerateChallenge()
	if !s.WriteSyncFrames(ctx,
		frame.NewChallenge("MD5", auth.ChallengeToString(s.Challenge)),
		frame.NewNull("SYS "+s.Config.System),
		frame.NewNull("ZYZ "+s.Config.Admin),
		frame.NewNull("LOC "+s.Config.Location),
		frame.NewNull("VER ginko/0.0.1/OpenBSD/x86_64 binkp/1.0"),
		frame.NewNull("TIME "+time.Now().Format(time.RFC1123Z)),
		frame.NewAddress(s.Config.Addresses()...)) {
		return session.End(ctx, s, errors.New("Error sending initial frames"))
	}
	return waitForAddress, nil
}

func waitForAddress(ctx context.Context, s *session.Session) (session.State, error) {
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
			log.Println("received ADR:", frame)
			if s.LinkAddresses(frame.Addresses()) == nil {
				err := errors.New("Unlinked session")
				log.Println(err)
				return session.End(ctx, s, err)
			}
			return waitForPasswd, nil
		case *frame.ErrorCmd:
			err := fmt.Errorf("received ERR: %v", frame)
			log.Println(err)
			return session.End(ctx, s, err)
		case *frame.BusyCmd:
			err := fmt.Errorf("received BUSY: %v", frame)
			log.Println(err)
			return session.End(ctx, s, err)
		case *frame.NullCmd:
			log.Println("received NUL:", frame)
		case *frame.OptCmd:
			log.Println("received OPT:", frame)
		default:
			err := fmt.Errorf("Found a weird frame: %v", frame)
			log.Println(err)
			s.SendErrorCmd(ctx, "Invalid received frame")
			return session.End(ctx, s, err)
		}
		return waitForAddress, nil
	}
}

func waitForPasswd(ctx context.Context, s *session.Session) (session.State, error) {
	select {
	case <-ctx.Done():
		return session.End(ctx, s, nil)
	case f, ok := <-s.ReadFrames:
		if ctx.Err() != nil || f == nil || !ok {
			err := errors.New("Error waiting for password")
			log.Println(err)
			return session.End(ctx, s, err)
		}
		switch frame := f.(type) {
		case *frame.PasswdCmd:
			log.Println("received PWD:", frame)
			return checkPasswd(ctx, s, frame.Password)
		case *frame.ErrorCmd:
			err := fmt.Errorf("received ERR: %v", frame)
			log.Println(err)
			return session.End(ctx, s, err)
		case *frame.BusyCmd:
			err := fmt.Errorf("received BUSY: %v", frame)
			log.Println(err)
			return session.End(ctx, s, err)
		case *frame.NullCmd:
			log.Println("received NUL:", frame)
		case *frame.OptCmd:
			log.Println("received OPT:", frame)
		default:
			err := fmt.Errorf("Found a weird frame: %v", frame)
			log.Println(err)
			s.SendErrorCmd(ctx, "Invalid received frame")
			return session.End(ctx, s, err)
		}
		return waitForPasswd, nil
	}
}

func checkPasswd(ctx context.Context, s *session.Session, password string) (session.State, error) {
	if password == "-" {
		err := errors.New("Unsuppored empty password")
		log.Println(err)
		s.SendErrorCmd(ctx, "Empty passwords are unsupported")
		return session.End(ctx, s, err)
	}
	if !strings.HasPrefix(password, "CRAM-") {
		err := errors.New("Unsupported cleartext password")
		log.Println(err)
		s.SendErrorCmd(ctx, "Cleartext passwords are unsupported")
		return session.End(ctx, s, err)
	}
	fields := strings.Split(password, "-")
	if len(fields) != 3 {
		err := fmt.Errorf("Malformed challenge response: %v", password)
		log.Println(err)
		s.SendErrorCmd(ctx, "Malformed challenge response")
		return session.End(ctx, s, err)
	}
	hashStr := fields[1]
	password = fields[2]
	if !auth.ValidateResponse(hashStr, s.Challenge, password, s.Link.Password) {
		err := errors.New("Password validation failed")
		log.Println(err)
		s.SendErrorCmd(ctx, "Invalid password")
		return session.End(ctx, s, err)
	}
	if !s.WriteSyncFrame(ctx, frame.NewOk("secure")) {
		return session.End(ctx, s, errors.New("Write Ok frame failed"))
	}
	return transfer.Start, nil
}
