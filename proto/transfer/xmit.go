package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"fat-dragon.org/ginko/frame"
	"fat-dragon.org/ginko/proto/session"
	"fat-dragon.org/ginko/spool"
)

type queueStatus int

const (
	queuePending queueStatus = iota
	queueSkipped
	queueDone
)

type xmitrSession struct {
	*session.Session
	spool   *spool.Spool
	lookup  map[spool.FileKey]*queueEntry
	queue   []*spool.SpoolKey
	active  []*xferDescr
	pending int
	request *xferDescr
}

type queueEntry struct {
	spoolKey *spool.SpoolKey
	status   queueStatus
}

func makeXmitrSession(outSpool *spool.Spool, s *session.Session) *xmitrSession {
	return &xmitrSession{
		s,
		outSpool,
		make(map[spool.FileKey]*queueEntry),
		nil,
		nil,
		0,
		nil,
	}
}
func (s *xmitrSession) loadQueue() error {
	if err := s.spool.ConsumeAndConcatQueues("new", "Queue", "cur", "Queue"); err != nil {
		return err
	}
	queue, err := s.spool.ReadQueue("cur", "Queue")
	if err != nil {
		return err
	}
	s.pending = len(queue)
	s.queue = make([]*spool.SpoolKey, len(queue))
	s.active = make([]*xferDescr, len(queue))
	for i, entry := range queue {
		entry := entry
		s.queue[i] = &entry
		s.lookup[entry.ToFileKey()] = &queueEntry{&entry, queuePending}
		s.active[i] = s.xferDescrFromSpoolKey(&entry, 0)
	}
	return nil
}

func (q *xmitrSession) xferDescrFromSpoolKey(key *spool.SpoolKey, offset int64) *xferDescr {
	return &xferDescr{
		key.ToFileKey(),
		offset,
		q.spool,
		key,
		nil,
	}
}

func makeKeyFromQueueingFrame(f frame.Queueing) *spool.FileKey {
	var key spool.FileKey
	switch qf := f.(type) {
	case *frame.GetCmd:
		key = spool.NewFileKey(qf.FileName, qf.Size, qf.TimeStamp)
	case *frame.GotCmd:
		key = spool.NewFileKey(qf.FileName, qf.Size, qf.TimeStamp)
	case *frame.SkipCmd:
		key = spool.NewFileKey(qf.FileName, qf.Size, qf.TimeStamp)
	default:
		panic("makeKeyFromQueueingFrame: input frame type error")
	}
	return &key
}

func (s *xmitrSession) get(key *spool.FileKey, offset int64) {
	log.Println("remote GET:", *key, "offset:", offset)
	qEntry, ok := s.lookup[*key]
	if !ok {
		log.Println("Queue entry not found for", *key)
		return
	}
	qEntry.status = queuePending
	s.removeFromActive(key)
	s.active = append(s.active, s.xferDescrFromSpoolKey(qEntry.spoolKey, offset))
	s.request = s.active[0]
}

func (q *xmitrSession) got(key *spool.FileKey) {
	log.Println("remote GOT:", *key)
	qEntry, ok := q.lookup[*key]
	if !ok {
		log.Println("Queue entry not found for", *key)
		return
	}
	qEntry.status = queueDone
	q.spool.Remove("cur", qEntry.spoolKey)
	q.removeFromActive(key)
	q.pending--
}

func (q *xmitrSession) skip(key *spool.FileKey) {
	log.Println("remote SKIP:", *key)
	qEntry, ok := q.lookup[*key]
	if !ok {
		log.Println("Queue entry not found for", *key)
		return
	}
	qEntry.status = queueSkipped
	q.removeFromActive(key)
	q.pending--
}

func (q *xmitrSession) removeFromActive(key *spool.FileKey) {
	newActive := make([]*xferDescr, 0)
	for _, entry := range q.active {
		if key.Equal(entry.spoolKey) {
			continue
		}
		newActive = append(newActive, entry)
	}
	q.active = newActive
}

func (q *xmitrSession) put() error {
	newQueue := []spool.SpoolKey{}
	for _, v := range q.queue {
		qEntry, ok := q.lookup[v.FileKey]
		if !ok || qEntry.status == queueDone {
			continue
		}
		newQueue = append(newQueue, *v)
	}
	return q.spool.SaveQueue("cur", "Queue", newQueue)
}

type xmitrState func(context.Context, *xmitrSession) (xmitrState, error)

func runXmitr(ctx context.Context, s *session.Session) error {
	defer close(s.XmitrDone)
	aux := makeXmitrSession(&s.Link.OutSpool, s)
	for state, err := startXmitr, error(nil); state != nil; {
		state, err = state(ctx, aux)
		if err != nil {
			return err
		}
	}
	return nil
}

func startXmitr(ctx context.Context, s *xmitrSession) (xmitrState, error) {
	if err := s.loadQueue(); err != nil {
		return xmitEnd, err
	}
	return xmitSendNextRequest, nil
}

func xmitFinishRequest(s *xmitrSession) {
	s.request.close()
	if len(s.active) > 0 {
		s.active = s.active[1:]
	}
}

func xmitSendNextRequest(ctx context.Context, s *xmitrSession) (xmitrState, error) {
	if s.pending <= 0 {
		return xmitEnd, nil
	}
	if len(s.active) == 0 {
		return xmitWaitForRequest, nil
	}
	s.request = s.active[0]
	return xmitSend, nil
}

func xmitWaitForRequest(ctx context.Context, s *xmitrSession) (xmitrState, error) {
	select {
	case <-ctx.Done():
		return xmitEnd, nil
	case f, ok := <-s.XmitrFrames:
		if ctx.Err() != nil || f == nil || !ok {
			return xmitEnd, nil
		}
		key := makeKeyFromQueueingFrame(f)
		switch frame := f.(type) {
		case *frame.GetCmd:
			s.get(key, frame.Offset)
			return xmitSend, nil
		case *frame.GotCmd:
			s.got(key)
		case *frame.SkipCmd:
			s.skip(key)
		default:
			panic("q.run: frame type error")
		}
		return xmitSend, nil
	}
	return xmitEnd, nil
}

func xmitSend(ctx context.Context, s *xmitrSession) (xmitrState, error) {
	if s.request == nil {
		return nil, errors.New("xmitSendRequest: nil request descriptor")
	}
	defer xmitFinishRequest(s)
	if err := s.request.open(); err != nil {
		// Failure to open the file probably means it was already
		// sent, but there was some sort of fault before we rewrote
		// the queue.  Treat this as if a `GOT` message had been
		// received.
		s.got(&s.request.FileKey)
		return xmitSendNextRequest, nil
	}
	fileCmd := s.request.fileCmd()
	log.Println("sending:", fileCmd)
	if !s.WriteFrame(ctx, s.request.fileCmd()) {
		return xmitSendNextRequest, nil
	}
	for !s.request.xferComplete() {
		select {
		case <- ctx.Done():
			return xmitEnd, nil
		case f, ok := <-s.XmitrFrames:
			if ctx.Err() != nil || f == nil || !ok {
				return xmitEnd, nil
			}
			key := makeKeyFromQueueingFrame(f)
			switch frame := f.(type) {
			case *frame.GetCmd:
				s.get(key, frame.Offset)
			case *frame.GotCmd:
				s.got(key)
			case *frame.SkipCmd:
				s.skip(key)
			default:
				panic("q.run: frame type error")
			}
			if key.Equal(s.request.spoolKey) {
				return xmitSendNextRequest, nil
			}
		default:
			dataFrame, err := frame.ReadDataFrameFrom(s.request.spoolFile, s.request.offset)
			if err != nil && err != io.EOF {
				return xmitEnd, fmt.Errorf("read error: %v", err)
			}
			log.Println("sending:", dataFrame.String())
			s.request.incrOffset(dataFrame.Length())
			if !s.WriteFrame(ctx, dataFrame) {
				break
			}
		}
	}
	s.FlushWriter(ctx)
	return xmitSendNextRequest, nil
}

func xmitEnd(ctx context.Context, s *xmitrSession) (xmitrState, error) {
	s.WriteSyncFrame(ctx, frame.NewEOB())
	s.put()
	log.Println("Transfer: sender exiting")
	return nil, nil
}
