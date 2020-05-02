package session

import (
	"context"
	"net"

	"fat-dragon.org/ginko/config"
	"fat-dragon.org/ginko/frame"
	"fat-dragon.org/ginko/ftn"
	"golang.org/x/sync/errgroup"
)

type Session struct {
	Config      *config.Config
	Link        *config.Link
	RemoteAddrs []ftn.Address
	HashStr     string
	Challenge   []byte
	urgentErr   chan frame.Terminal
	ReadFrames  chan frame.Frame
	writeFrames chan frame.Frame
	RecvrFrames chan frame.Frame
	RecvrDone   chan struct{}
	XmitrFrames chan frame.Queueing
	XmitrDone   chan struct{}
	waiter      *errgroup.Group
}

const readBufferSize = (32767 + 2) * 2

// NewSession constructs a new BINKP session and returns a
// session object.
func NewSession(ctx context.Context, config *config.Config, conn net.Conn) Session {
	waiter, ctx := errgroup.WithContext(ctx)
	urgentErr, readFrames := makeFrameReader(ctx, waiter, conn)
	writeFrames := makeFrameWriter(ctx, waiter, conn, urgentErr)
	recvrFrames := make(chan frame.Frame)
	recvrDone := make(chan struct{})
	xmitrFrames := make(chan frame.Queueing)
	xmitrDone := make(chan struct{})
	return Session{
		config,
		nil,
		nil,
		"MD5",
		nil,
		urgentErr,
		readFrames,
		writeFrames,
		recvrFrames,
		recvrDone,
		xmitrFrames,
		xmitrDone,
		waiter,
	}
}

// WriteSyncFrames synchronously writes and flushes a sequence of frames to
// the session writer.
func (s *Session) WriteSyncFrames(ctx context.Context, frames ...frame.Frame) bool {
	return s.WriteFrames(ctx, frames...) && s.FlushWriter(ctx)
}

// WriteFrames writes a sequence of frames into the session's writer.
func (s *Session) WriteFrames(ctx context.Context, frames ...frame.Frame) bool {
	for _, f := range frames {
		if !s.WriteFrame(ctx, f) {
			return false
		}
	}
	return true
}

// WriteSyncFrame synchronously writes and flushes a frame.
func (s *Session) WriteSyncFrame(ctx context.Context, f frame.Frame) bool {
	return s.WriteFrame(ctx, f) && s.FlushWriter(ctx)
}

// WriteFrame writes a frame into the session's writer.
func (s *Session) WriteFrame(ctx context.Context, f frame.Frame) bool {
	select {
	case <-ctx.Done():
		return false
	case s.writeFrames <- f:
		if ctx.Err() != nil {
			return false
		}
		return true
	}
}

// FlushWriter sends an out-of-band "Flush" message to the writer to flush its
// state.
func (s *Session) FlushWriter(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case s.writeFrames <- nil:
		if ctx.Err() != nil {
			return false
		}
		return true
	}
}

// SendErrorCmd sends an ErrorCmd to the writer.
func (s *Session) SendErrorCmd(ctx context.Context, text string) bool {
	select {
	case <-ctx.Done():
		return false
	case s.urgentErr <- frame.NewErrorCmd(text):
		if ctx.Err() != nil {
			return false
		}
		return true
	}
}

// End ends a session.
func End(_ context.Context, _ *Session, err error) (State, error) {
	return nil, err
}

// Run starts a session at the initial state.
func (s *Session) Run(ctx context.Context, initState State) error {
	s.waiter.Go(func() error {
		for state := initState; state != nil; {
			var err error
			state, err = state(ctx, s)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return s.Wait()
}

// LinkAddresses looks for a link related to the given address and
// sets the `Link` member appropriately if it finds one.  It returns
// the link or nil.
func (s *Session) LinkAddresses(addrs []ftn.Address) *config.Link {
	s.RemoteAddrs = addrs
	for _, addr := range s.RemoteAddrs {
		if link := s.Config.Links[addr]; link != nil {
			s.Link = link
			return link
		}
	}
	return nil
}

// Wait waits for the session to end.
func (s *Session) Wait() error {
	return s.waiter.Wait()
}
