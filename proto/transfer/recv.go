package transfer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"fat-dragon.org/ginko/frame"
	"fat-dragon.org/ginko/proto/session"
)

type recvrSession struct {
	*session.Session
	request *xferDescr
}

type recvrState func(context.Context, *recvrSession) (recvrState, error)

func runRecvr(ctx context.Context, s *session.Session) error {
	defer close(s.RecvrDone)
	aux := &recvrSession{s, nil}
	for state, err := waitForFile, error(nil); state != nil; {
		state, err = state(ctx, aux)
		if err != nil {
			return err
		}
	}
	return nil
}

func waitForFile(ctx context.Context, s *recvrSession) (recvrState, error) {
	select {
	case <-ctx.Done():
		return recvEnd(ctx, nil)
	case f, ok := <-s.RecvrFrames:
		if ctx.Err() != nil || f == nil || !ok {
			err := errors.New("Error waiting for FILE")
			log.Println(err)
			return recvEnd(ctx, err)
		}
		switch frame := f.(type) {
		case *frame.FileCmd:
			return fileRecvRequested(ctx, s, NewXferDescr(frame))
		case *frame.EOBCmd:
			return recvEnd(ctx, nil)
		case *frame.Data:
			log.Println("received:", frame)
		default:
			err := fmt.Errorf("Found a weird frame: %v", frame)
			log.Println(err)
			s.SendErrorCmd(ctx, "Invalid received frame")
			return recvEnd(ctx, err)
		}
		return waitForFile, nil
	}
}

func fileRecvRequested(ctx context.Context, s *recvrSession, request *xferDescr) (recvrState, error) {
	s.request = request
	if hasFile(request) {
		return gotFile, nil
	}
	request.spool = &s.Link.InSpool
	spoolKey, file, err := request.spool.TempFileFor(&request.FileKey)
	if err != nil {
		return recvEnd(ctx, err)
	}
	request.spoolFile = file
	request.spoolKey = spoolKey
	return recvFileData, nil
}

func hasFile(_ *xferDescr) bool {
	return false
}

func recvFileData(ctx context.Context, s *recvrSession) (recvrState, error) {
	select {
	case <-ctx.Done():
		return recvEnd(ctx, nil)
	case f, ok := <-s.RecvrFrames:
		if ctx.Err() != nil || f == nil || !ok {
			err := errors.New("Error waiting for file data")
			log.Println(err)
			return recvEnd(ctx, err)
		}
		switch frame := f.(type) {
		case *frame.Data:
			return recvDataFrame(ctx, s, frame)
		case *frame.FileCmd:
			log.Println("FILE received:", frame, ", previous file incomplete:", s.request)
			s.request.abort()
			s.request = nil
			return fileRecvRequested(ctx, s, NewXferDescr(frame))
		default:
			s.SendErrorCmd(ctx, "Invalid received frame")
			err := fmt.Errorf("Found a weird frame: %f", frame)
			log.Println(err)
			return recvEnd(ctx, err)
		}
		return waitForFile, nil
	}
}

func recvDataFrame(ctx context.Context, s *recvrSession, frame *frame.Data) (recvrState, error) {
	descr := s.request
	data := frame.Data()
	dataLen := int64(len(data))
	if descr.offset+dataLen > descr.Size {
		log.Printf("long write %v: %v", descr, frame)
	}
	nb, err := descr.spoolFile.WriteAt(data, descr.offset)
	if err != nil || nb != len(data) {
		err := fmt.Errorf("error writing receive file %v: %v", descr.spoolFile, err)
		log.Println(err)
		descr.abort()
		return recvEnd(ctx, err)
	}
	descr.incrOffset(nb)
	if !descr.xferComplete() {
		return recvFileData, nil
	}
	return gotFile, nil
}

func gotFile(ctx context.Context, s *recvrSession) (recvrState, error) {
	descr := s.request
	s.request = nil
	if err := descr.publish(); err != nil {
		err := fmt.Errorf("spool publish error: %v", err)
		log.Println(err)
		return recvError(ctx, s, err)
	}
	if !s.WriteSyncFrame(ctx, frame.NewGot(descr.FileName, descr.Size, time.Unix(descr.TimeStamp, 0))) {
		return recvEnd(ctx, errors.New("Error writing GOT frame"))
	}
	return waitForFile, nil
}

func recvError(ctx context.Context, s *recvrSession, err error) (recvrState, error) {
	s.SendErrorCmd(ctx, "internal server error")
	return recvEnd(ctx, errors.New("internal server error"))
}

func recvEnd(_ context.Context, err error) (recvrState, error) {
	log.Println("Transfer: receiver exiting")
	return nil, err
}
