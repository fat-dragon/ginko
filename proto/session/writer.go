package session

import (
	"bufio"
	"context"
	"fmt"
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
func makeFrameWriter(ctx context.Context, waiter *errgroup.Group, conn net.Conn, urgentErr chan frame.Terminal) chan frame.Frame {
	frames := make(chan frame.Frame, 16)
	waiter.Go(func() error {
		const writeBufferSize = (32767 + 2) * 4
		writer := bufio.NewWriterSize(conn, writeBufferSize)
		for {
			select {
			case errorFrame, ok := <-urgentErr:
				if !ok || ctx.Err() != nil {
					return nil
				}
				if err := errorFrame.WriteBytes(writer); err != nil {
					return fmt.Errorf("Error writing frame: %v", err)
				}
				return writer.Flush()
			case f, ok := <-frames:
				if !ok || ctx.Err() != nil {
					return nil
				}
				if f == nil {
					if err := writer.Flush(); err != nil {
						return fmt.Errorf("Error flushing frames: %v", err)
					}
				} else if err := f.WriteBytes(writer); err != nil {
					return fmt.Errorf("Error writing frame: %v", err)
				}
			case <-ctx.Done():
				return nil
			}
		}
	})
	return frames
}
