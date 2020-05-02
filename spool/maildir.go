package spool

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"time"
)

var seqC chan uint64
var pid int
var hostname string
var startSeqNo uint64
var randData [16]byte
var randKey [16]byte

func init() {
	copy(randData[:], []byte("deadbeefcafef00d"))
	pid = os.Getpid()
	var err error
	hostname, err = os.Hostname()
	if err != nil {
		panic(err)
	}
	startSeqNo, err = fetchAndIncrStartSeqNo()
	if err != nil {
		panic(err)
	}
	seqC = make(chan uint64)
	go func() {
		seqNo := uint64(0)
		for {
			seqC <- seqNo
			seqNo++
		}
	}()
}

func fetchAndIncrStartSeqNo() (uint64, error) {
	u, err := user.Current()
	if err != nil {
		return 0, err
	}
	startSeqFileName := path.Join(u.HomeDir, ".ginkoseq")
	f, err := openLocked(startSeqFileName)
	if err != nil {
		return 0, err
	}
	defer closeLocked(f)
	bs, err := ioutil.ReadAll(f)
	if err != nil {
		return 0, err
	}
	seqStr := strings.TrimSpace(string(bs))
	if seqStr == "" {
		seqStr = "0"
	}
	seq, err := strconv.ParseUint(seqStr, 10, 64)
	if err != nil {
		return 0, err
	}
	seq += 1
	if err := f.Truncate(0); err != nil {
		return 0, err
	}
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return 0, err
	}
	if _, err := fmt.Fprintf(f, "%v\n", seq); err != nil {
		return seq, err
	}
	return seq, nil
}

func uniqueName() string {
	seqNo := <-seqC
	r, _ := cryptoRand(seqNo)
	now := time.Now()
	return fmt.Sprintf("%v.X%xR%sM%vP%dQ%v.%s",
		now.Unix(), startSeqNo, r, now.UnixNano()/1000, pid, seqNo, hostname)
}

func cryptoRand(seqNo uint64) (string, error) {
	// Reseed the HMAC every 1000 iterations.
	if seqNo%1000 == 0 {
		rand.Read(randKey[:])
	}
	mac := hmac.New(md5.New, randKey[:])
	mac.Write(randData[:])
	copy(randData[:], mac.Sum(nil))
	return hex.EncodeToString(randData[:]), nil
}
