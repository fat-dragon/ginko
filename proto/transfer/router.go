package transfer

import (
	"context"
	"errors"
	"fmt"
	"log"

	"fat-dragon.org/ginko/frame"
	"fat-dragon.org/ginko/proto/session"
)

func runRouter(ctx context.Context, s *session.Session) error {
	defer close(s.RecvrFrames)
	defer close(s.XmitrFrames)
	for state, err := router, error(nil); state != nil; {
		state, err = state(ctx, s)
		if err != nil {
			return err
		}
	}
	return nil
}

func router(ctx context.Context, s *session.Session) (session.State, error) {
	var f frame.Frame
	var ok bool
	select {
	case <-ctx.Done():
		return routerEnd(nil)
	case f, ok = <-s.ReadFrames:
		if ctx.Err() != nil || f == nil || !ok {
			err := errors.New("Router: error waiting for frame")
			log.Println(err)
			return routerEnd(err)
		}
	case <-s.RecvrDone:
		return routeXmitr, nil
	case <-s.XmitrDone:
		return routeRecvr, nil
	}
	switch frame := f.(type) {
	case *frame.FileCmd:
		log.Println("received:", frame)
		s.RecvrFrames <- frame
	case *frame.BusyCmd:
		log.Println("received:", frame)
	case *frame.ErrorCmd:
		log.Println("received:", frame)
	case *frame.EOBCmd:
		log.Println("received:", frame)
		s.RecvrFrames <- frame
	case *frame.Data:
		log.Println("received:", frame)
		s.RecvrFrames <- frame
	case *frame.NullCmd:
		log.Println("received:", frame)
	case *frame.OptCmd:
		log.Println("received:", frame)
	case *frame.GetCmd:
		log.Println("received:", frame)
		s.XmitrFrames <- frame
	case *frame.GotCmd:
		log.Println("received:", frame)
		s.XmitrFrames <- frame
	case *frame.SkipCmd:
		log.Println("received:", frame)
		s.XmitrFrames <- frame
	default:
		err := fmt.Errorf("Found a weird frame: %v", frame)
		log.Println(err)
		s.SendErrorCmd(ctx, "Invalid received frame")
		return routerEnd(err)
	}
	return router, nil
}

func routeXmitr(ctx context.Context, s *session.Session) (session.State, error) {
	var f frame.Frame
	var ok bool
	select {
	case <-ctx.Done():
		return routerEnd(nil)
	case f, ok = <-s.ReadFrames:
		if ctx.Err() != nil || f == nil || !ok {
			err := errors.New("Router: error waiting for frame")
			log.Println(err)
			return routerEnd(err)
		}
	case <-s.XmitrDone:
		return routerEnd(nil)
	}
	switch frame := f.(type) {
	case *frame.NullCmd:
		log.Println("received:", frame)
	case *frame.OptCmd:
		log.Println("received:", frame)
	case *frame.BusyCmd:
		log.Println("received:", frame)
		return routerEnd(nil)
	case *frame.ErrorCmd:
		log.Println("received:", frame)
		return routerEnd(nil)
	case *frame.GetCmd:
		log.Println("received:", frame)
		s.XmitrFrames <- frame
	case *frame.GotCmd:
		log.Println("received:", frame)
		s.XmitrFrames <- frame
	case *frame.SkipCmd:
		log.Println("received:", frame)
		s.XmitrFrames <- frame
	default:
		err := fmt.Errorf("Found a weird frame: %v", frame)
		log.Println(err)
		s.SendErrorCmd(ctx, "Invalid received frame")
		return routerEnd(err)
	}
	return routeXmitr, nil
}

func routeRecvr(ctx context.Context, s *session.Session) (session.State, error) {
	var f frame.Frame
	var ok bool
	select {
	case <-ctx.Done():
		return routerEnd(nil)
	case f, ok = <-s.ReadFrames:
		if ctx.Err() != nil || f == nil || !ok {
			err := errors.New("Router: error waiting for frame")
			log.Println(err)
			return routerEnd(err)
		}
	case <-s.RecvrDone:
		return routerEnd(nil)
	}
	switch frame := f.(type) {
	case *frame.NullCmd:
		log.Println("received:", frame)
	case *frame.OptCmd:
		log.Println("received:", frame)
	case *frame.BusyCmd:
		log.Println("received:", frame)
		return routerEnd(nil)
	case *frame.ErrorCmd:
		log.Println("received:", frame)
		return routerEnd(nil)
	case *frame.FileCmd:
		log.Println("received:", frame)
		s.RecvrFrames <- frame
	case *frame.EOBCmd:
		log.Println("received:", frame)
		s.RecvrFrames <- frame
	case *frame.Data:
		log.Println("received:", frame)
		s.RecvrFrames <- frame
	default:
		err := fmt.Errorf("Found a weird frame: %v", frame)
		log.Println(err)
		s.SendErrorCmd(ctx, "Invalid received frame")
		return routerEnd(err)
	}
	return routeRecvr, nil
}

func routerEnd(err error) (session.State, error) {
	log.Println("Frame router exiting")
	return nil, err
}
