package transfer

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"fat-dragon.org/ginko/frame"
	"fat-dragon.org/ginko/proto/session"
	"fat-dragon.org/ginko/spool"
	"golang.org/x/sync/errgroup"
)

type state func(ctx context.Context, s *recvrState) (state, error)

func Start(ctx context.Context, s *session.Session) (session.State, error) {
	g, egCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return runRouter(egCtx, s)
	})
	g.Go(func() error {
		return runRecvr(egCtx, s)
	})
	g.Go(func() error {
		return runXmitr(egCtx, s)
	})
	return session.End(ctx, s, g.Wait())
}

type xferDescr struct {
	spool.FileKey
	offset    int64
	spool     *spool.Spool
	spoolKey  *spool.SpoolKey
	spoolFile *os.File
}

func (d xferDescr) String() string {
	return fmt.Sprintf("transfer descriptor: file: %q, size %v, time stamp \"%v\", offset %v",
		d.FileName, d.Size, d.TimeStamp, d.offset)
}

func (d *xferDescr) fileKeyEquals(key *spool.FileKey) bool {
	return d.FileName == key.FileName &&
		d.Size == key.Size &&
		d.TimeStamp == key.TimeStamp
}

func (d *xferDescr) fileCmd() *frame.FileCmd {
	return frame.NewFileCmd(d.FileKey.FileName, d.FileKey.Size, time.Unix(d.FileKey.TimeStamp, 0), d.offset)
}

func (d *xferDescr) open() error {
	spoolFile := d.spool.FileName("cur", d.spoolKey.Name)
	file, err := os.Open(spoolFile)
	if err != nil {
		return err
	}
	if _, err := file.Seek(d.offset, io.SeekStart); err != nil {
		return err
	}
	d.spoolFile = file
	return nil
}

func (d *xferDescr) close() error {
	return d.spoolFile.Close()
}

func (d *xferDescr) abort() {
	d.close()
	d.spool.Abort(d.spoolKey)
}

func (d *xferDescr) publish() error {
	if err := d.spoolFile.Sync(); err != nil {
		d.abort()
		return err
	}
	if err := d.close(); err != nil {
		d.abort()
		return err
	}
	if err := d.spool.Publish(d.spoolKey); err != nil {
		return err
	}
	return nil
}

func (d *xferDescr) xferComplete() bool {
	return d.offset >= d.Size
}

func (d *xferDescr) incrOffset(n int) {
	d.offset += int64(n)
}

func NewXferDescr(fileCmd *frame.FileCmd) *xferDescr {
	fileKey := spool.NewFileKey(fileCmd.FileName, fileCmd.Size, fileCmd.TimeStamp)
	return &xferDescr{fileKey, fileCmd.Offset, nil, nil, nil}
}
