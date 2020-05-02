package session

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"fat-dragon.org/ginko/frame"
	"golang.org/x/sync/errgroup"
)

// Starts a new frame reader wrapping a network connection in a
// goroutine.  Takes a context, a network connection and a write
// chan of ErrorCmds for dispatching an error frame to the distant
// end.
//
// Returns a read chan of frames, as well as an error chan and
// possibly an error for detecting problems at startup.
func makeFrameReader(ctx context.Context, waiter *errgroup.Group, conn net.Conn) (chan frame.Terminal, chan frame.Frame) {
	urgentErr := make(chan frame.Terminal)
	out := make(chan frame.Frame, 16)
	waiter.Go(func() error {
		const readBufferSize = (32767 + 2) * 4
		defer close(urgentErr)
		defer close(out)
		reader := bufio.NewReaderSize(conn, readBufferSize)
		for {
			f, err := readFrame(reader)
			if err != nil {
				if err == io.EOF {
					break
				}
				msg := fmt.Sprintf("Error reading frame: %v", err)
				urgentErr <- frame.NewErrorCmd(msg)
				return errors.New(msg)
			}
			select {
			case out <- f:
				if ctx.Err() != nil {
					break
				}
			case <-ctx.Done():
				break
			}
		}
		return nil
	})
	return urgentErr, out
}

// Reads a frame from the given reader.  Handles empty and unknown frames.
func readFrame(reader io.Reader) (frame.Frame, error) {
	const maxConsecutiveBadFrames = 1000
	for i := 0; i < maxConsecutiveBadFrames; i++ {
		f, err := frame.Read(reader)
		if err != nil {
			return nil, err
		}
		if f == nil {
			continue
		}
		return f, nil
	}
	return nil, errors.New("Too many consecutive bad frames")
}
