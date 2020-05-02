package spool

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"
)

// FileKey uniquely identifies a file: it consists
// of a name, size and timestamp (seconds since the
// Unix epoch).
type FileKey struct {
	FileName  string
	Size      int64
	TimeStamp int64
}

func NewFileKey(name string, size int64, timeStamp time.Time) FileKey {
	return FileKey{name, size, timeStamp.Unix()}
}

// Equals compares two file keys for equality.
func (k *FileKey) Equal(other *SpoolKey) bool {
	return *k == other.FileKey
}

// SpoolKey identifies a file in the spool.  It
// contains a FileKey and a Name, which is the
// maildir name of the actual spooled file.  This
// record is what is stored in the queue.
type SpoolKey struct {
	Name      string
	SpoolTime time.Time
	FileKey   FileKey
}

// ToFileKey returns the FileKey associated with a SpoolKey.
func (k *SpoolKey) ToFileKey() FileKey {
	return k.FileKey
}

// Queue is a type representing a queue of spooled
// files to be processed.
type Queue []SpoolKey

// Spool is a wrapper around a Maildir.
type Spool struct {
	baseDir string
}

// TempFile creates a new file in the spool's `tmp` directory
// to store incoming data.  A SpoolKey and the opened file
// are returned; the file is advisory locked.
func (s *Spool) TempFileFor(fileKey *FileKey) (spoolKey *SpoolKey, file *os.File, err error) {
	for i := 0; i < 1000; i++ {
		tmpName := uniqueName()
		pathname := s.FileName("tmp", tmpName)
		file, err = openLocked(pathname)
		if err == nil {
			spoolKey = &SpoolKey{tmpName, time.Now(), *fileKey}
			break
		}
	}
	return
}

// Publish takes a spool key and "publishes" it into the
// maildir's "new" directory and work queue.
//
// This code is tricky, as the ordering of operations
// involved is important to durability in the face of
// crashes and faults.
//
// The basic sequence of events is:
// 1. Remove the uniquename from `new` to clean up from
//    earlier failures
// 2. Link the uniquename from `tmp` to `new`
// 3. Open and lock the mutex file
// 4. Read the current queue
// 5. Append the key for this delivery to the queue.
// 6. Write the new queue to a temporary file
// 7. Atomically rename the temporary queue uniquename
//    to the queue file's name
// 8. Unlink the file's uniquename in tmp
// 9. Truncate, unlock and close the mutex fil.e
func (s *Spool) Publish(spoolKey *SpoolKey) error {
	var err error

	tmpName := s.FileName("tmp", spoolKey.Name)
	pubName := s.FileName("new", spoolKey.Name)
	mutexName := s.FileName("new", "Mutex")

	// Remove pubName if it exists; clean up earlier failures.
	os.Remove(pubName)

	// Link the temporary file to the published file name.
	os.Link(tmpName, pubName)

	// Open and lock the Mutex file.  The Mutex exists
	// to prevent a consumer examining the queue file
	// while we are appending an entry to it.
	m, err := openMutex(mutexName)
	if err != nil {
		return err
	}
	defer closeMutex(m)

	// Read the current queue into a slice of SpoolKeys.
	queue, err := s.ReadQueue("new", "Queue")
	if err != nil {
		return err
	}

	// Append the key to the queue.
	queue = append(queue, *spoolKey)

	// Save the new queue into a temporary file, and atomically
	// rename it to the published queue name.
	if err := s.SaveQueue("new", "Queue", queue); err != nil {
		return err
	}

	// Unlink the temp file.
	if err := os.Remove(tmpName); err != nil {
		return err
	}

	// We are done.  Returning here will implicitly
	// unlock and close the mutex file.
	return nil
}

// Abort deletes the tmp file associated with the given
// SpoolKey.
func (s *Spool) Abort(key *SpoolKey) {
	os.Remove(s.FileName("tmp", key.Name))
}

// FileName returns the name of a file relative to the
// given spool.
func (s *Spool) FileName(dir string, name string) string {
	return path.Join(s.baseDir, dir, name)
}

// ReadQueue reads a queue from the given file.
func (s *Spool) ReadQueue(dir string, name string) (queue Queue, err error) {
	queueName := s.FileName(dir, name)
	f, err := os.OpenFile(queueName, os.O_RDONLY|os.O_CREATE, 0660)
	if err != nil {
		return
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}
	queue = make(Queue, 0)
	if len(data) > 0 {
		err = json.Unmarshal(data, &queue)
	}
	return
}

// SaveQueue writes a queue to the given file.
func (s *Spool) SaveQueue(dir string, name string, queue []SpoolKey) error {
	data, err := json.MarshalIndent(queue, "", " ")
	if err != nil {
		return err
	}
	tmpQueueName := s.FileName("tmp", uniqueName()+".Queue")
	file, err := os.OpenFile(tmpQueueName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0660)
	if err != nil {
		return err
	}
	defer file.Close()
	nbytes, err := file.Write(data)
	if err != nil {
		os.Remove(tmpQueueName)
		return err
	}
	if nbytes != len(data) {
		os.Remove(tmpQueueName)
		return errors.New("Short write to queue file")
	}
	_, err = file.WriteString("\n")
	if err != nil {
		os.Remove(tmpQueueName)
		return err
	}
	if err := file.Sync(); err != nil {
		os.Remove(tmpQueueName)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(tmpQueueName)
		return err
	}
	// Atomically replace the old queue with the new queue.
	queueName := s.FileName(dir, name)
	if err := os.Rename(tmpQueueName, queueName); err != nil {
		os.Remove(tmpQueueName)
		return err
	}
	return nil
}

// ConsumeAndConcatQueues takes two queues and combines them.
// This uses algorithms that ensure that failure at any stage
// is recoverable.
func (s *Spool) ConsumeAndConcatQueues(fromDir, fromName, toDir, toName string) error {
	if err := s.Consume(fromDir, fromName, toDir, "Incoming"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return s.ConcatQueues(toDir, "Incoming", toName)
}

// Consume atomically takes a published queue and moves it to a
// new directory.
func (s *Spool) Consume(fromDir, fromName, toDir, toName string) error {
	// Open and lock the Mutex file.  The Mutex exists
	// to prevent a consumer examining the queue file
	// while we are appending an entry to it.
	m, err := openMutex(s.FileName(fromDir, "Mutex"))
	if err != nil {
		return err
	}
	defer closeMutex(m)

	// Atomically move the "new" queue into the work
	// directory.
	outgoingName := s.FileName(fromDir, fromName)
	incomingName := s.FileName(toDir, toName)
	if err := os.Rename(outgoingName, incomingName); err != nil {
		return err
	}

	// We are done.  The Mutex will be automatically
	// released and the lock file closed.
	return nil
}

// ConcatQueues takes an incoming file name, a spool subdir
// and a queue name and concatenates any existing queue
// with the incoming queue, saving the results in such a
// manner that software can recover from a crash at any
// point.
func (s *Spool) ConcatQueues(toDir, inName, name string) error {
	// Read the old and new queues, concatenate them, and write
	// the result to `Staging`.  Note that `Staging` will exit
	// IFF the concatenation is safely stored.
	queue, err := s.ReadQueue(toDir, name)
	if err != nil {
		return err
	}
	newQueue, err := s.ReadQueue(toDir, inName)
	if err != nil {
		return err
	}
	for _, entry := range newQueue {
		fromName := s.FileName("new", entry.Name)
		toName := s.FileName("cur", entry.Name)
		if err := os.Rename(fromName, toName); err != nil {
			log.Println("Error moving file:", err)
			continue
		}
		queue = append(queue, entry)
	}
	newQueueName := s.FileName(toDir, "Staging")
	if err := s.SaveQueue(toDir, "Staging", queue); err != nil {
		os.Remove(newQueueName)
		return err
	}

	// At this point, `Staging` exists in the target
	// directory and contains the concatenation of the old
	// and new queues.  It is safe to remove the `Incoming`
	// file and rename `Staging` to `Queue`.
	os.Remove(s.FileName(toDir, inName))
	queueName := s.FileName(toDir, name)
	if err := os.Rename(newQueueName, queueName); err != nil {
		os.Remove(newQueueName)
		return err
	}

	// We are done.
	return nil
}

// Remove deletes a file from the spool.
func (s *Spool) Remove(dir string, key *SpoolKey) error {
	return os.Remove(s.FileName(dir, key.Name))
}

// Unmarshal a "spool" from a string in a JSON stream.  We
// parse the spool directory from the string and build a
// Spool from that.
func (s *Spool) UnmarshalJSON(data []byte) error {
	var baseDir string
	if err := json.Unmarshal(data, &baseDir); err != nil {
		return err
	}
	*s = Spool{baseDir}
	return nil
}
