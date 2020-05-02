package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)


type FTNHeader interface {
	isFTNHeader()
}

// FTNHeaderStoneAge is the byte-for-byte packet header format defined
// in FTS-0001.016.
type rawFTNHeaderStoneAge struct {
	OriginNode uint16
	DestNode   uint16
	Year       uint16
	Month      uint16
	Day        uint16
	Hour       uint16
	Minute     uint16
	Second     uint16
	BaudRate   uint16
	Magic      uint16
	OriginNet  uint16
	DestNet    uint16
	ProdCode   uint8
	SerialNo   uint8
	Password   [8]byte
	OriginZone uint16 // Marked option in FTS-0001.16
	DestZone   uint16 // Marked option in FTS-0001.16
	// Followed by 20 bytes of filler....
}

func (h rawFTNHeaderStoneAge) isFTNHeader() {}

// rawFTNHeader2e is the byte-for-byte packet header format defined
// in FSC-0039.004.  This differenst from FSC-0039.001 in that
// `CapValid` was taken out of the last two bytes of `Filler`.
type rawFTNHeader2e struct {
	OriginNode   uint16
	DestNode     uint16
	Year         uint16
	Month        uint16
	Day          uint16
	Hour         uint16
	Minute       uint16
	Second       uint16
	BaudRate     uint16
	PktVersion   uint16 // 0x02
	OriginNet    uint16
	DestNet      uint16
	ProdCodeLow  byte
	VersionMajor byte
	Password     [8]byte
	QOriginZone  uint16 // "ZmailQ,QMail"
	QDestZone    uint16 // "ZmailQ,QMail"
	Filler       [2]byte
	CapValid     uint16
	ProdCodeHigh byte
	VersionMinor byte
	CapWord      uint16
	OriginZone   uint16
	DestZone     uint16
	OriginPoint  uint16
	DestPoint    uint16
	ProdSpecData uint16
	PktTerm      uint16 // Must be zero.
}

func (h rawFTNHeader2e) isFTNHeader() {}

// rawFTNHeader22 is the byte-for-byte packet header format defined
// in FSC-0045.001.  It supports five-dimensional addressing.
type rawFTNHeader22 struct {
	OriginNode   uint16
	DestNode     uint16
	OriginPoint  uint16
	DestPoint    uint16
	ReservedMB0  [8]byte
	PktSubVers   uint16
	PktVersion   uint16
	OriginNet    uint16
	DestNet      uint16
	ProdCode     byte
	ProdRevLeve  byte
	Password     [8]byte
	OriginZone   uint16
	DestZone     uint16
	OriginDomain [8]byte
	DestDomain   [8]byte
	ProdSpecData [4]byte
}

func (h rawFTNHeader22) isFTNHeader() {}

// rawFTNHeader2p is the byte-for-byte packet header defined
// in FSC-0048.002.  It is claimed that it is the most common
// packet header format in common use.
//
// Packet types 2+ and 2e are largely compatible, except that
// 2+ contains the `AuxNet` field in lieu of 2e's `Filler`, and
// the product-specific data `ProdSpecData` is widened to 4 bytes,
// absorbing the mandatorily zero `PktTerm` packet terminator
// of 2e.
type rawFTNHeader2p struct {
	OriginNode   uint16
	DestNode     uint16
	Year         uint16
	Month        uint16
	Day          uint16
	Hour         uint16
	Minute       uint16
	Second       uint16
	BaudRate     uint16
	PktVersion   uint16 // 0x02
	OriginNet    uint16
	DestNet      uint16
	ProdCodeLow  byte
	VersionMajor byte
	Password     [8]byte
	QOriginZone  uint16 // "ZmailQ,QMail"
	QDestZone    uint16 // "ZmailQ,QMail"
	AuxNet       uint16 // Replaces 2e's `Filler` field
	CapValid     uint16
	ProdCodeHigh byte
	VersionMinor byte
	CapWord      uint16
	OriginZone   uint16  // "As in FD etc"
	DestZone     uint16  // "As in FD etc"
	OriginPoint  uint16  // "As in FD etc"
	DestPoint    uint16  // "As in FD etc"
	ProdSpecData [4]byte // aborbs 2e's `PktTerm` field.
}

func (h rawFTNHeader2p) isFTNHeader() {}

// MsgHeader is the message header format defined in FTS-0001.016.
type MsgHeader struct {
	OriginNode uint16
	DestNode   uint16
	OriginNet  uint16
	DestNet    uint16
	Attributes uint16
	Cost       uint16
}

// The actual contents of a message.
type Message struct {
	area       string
	from       string
	to         string
	subject    string
	dateTime   string
	kludges    []string
	tearLine   string
	originLine string
	body       []string
	seenBy     []string
	path       []string
}

func (message *Message) Print() {
	fmt.Println("DATE:", message.dateTime)
	fmt.Println("FROM:", message.from)
	fmt.Println("TO:  ", message.to)
	fmt.Println("SUBJ:", message.subject)
	fmt.Println("AREA:", message.area)
	fmt.Println("-->KLUDGES<--")
	for _, kludge := range message.kludges {
		fmt.Println(kludge)
	}
	fmt.Println("--->SEEN-BY<--")
	for _, seen := range message.seenBy {
		fmt.Println(seen)
	}
	fmt.Printf("ORIGIN LINE-->%q\n", message.originLine)
	fmt.Printf("TEAR LINE-->%q\n", message.tearLine)
	fmt.Println("-->BODY<--")
	for _, line := range message.body {
		fmt.Println(line)
	}
}

const dir = "/fat-dragon/trunk/ftn/fsxnet/in/new"

func main() {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal("some error occured:", err)
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := filepath.Join(dir, file.Name())
		visit(name)
	}
}

func visit(file string) {
	r, err := zip.OpenReader(file)
	if err != nil {
		log.Println("error opening", file, err)
		return
	}
	defer r.Close()

	for _, f := range r.File {
		if !strings.HasSuffix(f.Name, ".pkt") {
			continue
		}
		fmt.Printf("Contents of %s:\n", f.Name)
		rc, err := f.Open()
		if err != nil {
			log.Fatal("error opening", f.Name, err)
		}
		pr := bufio.NewReader(rc)
		var rawFTNHeader [58]byte
		if err := binary.Read(pr, binary.LittleEndian, &rawFTNHeader); err != nil {
			fmt.Println("binary.Read failed:", err)
		}
		for {
			var magic uint16
			if err := binary.Read(pr, binary.LittleEndian, &magic); err != nil {
				fmt.Println("Error.")
			}
			if magic == 0 {
				break
			}
			var msgHeader MsgHeader
			if err := binary.Read(pr, binary.LittleEndian, &msgHeader); err != nil {
				fmt.Println("binary.Read failed:", err)
			}
			fmt.Printf("Message Header: from %d/%d to %d/%d magic %d attr %d cost %d\n",
				msgHeader.OriginNode, msgHeader.OriginNet, msgHeader.DestNode, msgHeader.DestNet,
				magic, msgHeader.Attributes, msgHeader.Cost)
			var message Message
			dateTime, err := pr.ReadString(0)
			if err != nil {
				fmt.Println("Fail reading time:", err)
			}
			message.dateTime = strings.TrimSuffix(dateTime, "\x00")
			to, err := pr.ReadString(0)
			if err != nil {
				fmt.Println("Fail reading to:", err)
			}
			message.to = strings.TrimSuffix(to, "\x00")
			from, err := pr.ReadString(0)
			if err != nil {
				fmt.Println("Fail reading from:", err)
			}
			message.from = strings.TrimSuffix(from, "\x00")
			subject, err := pr.ReadString(0)
			if err != nil {
				fmt.Println("Fail reading subj:", err)
			}
			message.subject = strings.TrimSuffix(subject, "\x00")
			body, err := pr.ReadString(0)
			if err != nil {
				fmt.Println("Fail reading body:", err)
			}
			body = strings.TrimSuffix(body, "\x00")
			scanner := bufio.NewScanner(strings.NewReader(body))
			scanner.Split(splitPacketLine)
			if !scanner.Scan() {
				fmt.Println("Fail reading area")
			}
			area := scanner.Text()
			upperArea := strings.ToUpper(area)
			if !strings.HasPrefix(upperArea, "AREA:") {
				fmt.Println("Fail parsing area")
			}
			message.area = strings.TrimSpace(strings.TrimPrefix(upperArea, "AREA:"))
			kludges := []string{}
			lines := []string{}
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "\x01") {
					kludges = append(kludges, line[1:])
					continue
				}
				if strings.HasPrefix(line, " * Origin:") {
					message.originLine = line
				}
				if strings.HasPrefix(line, "---") {
					message.tearLine = line
				}
				lines = append(lines, line)
			}
			message.kludges = kludges
			i := len(lines)
			for i > 1 && isSeenBy(lines[i-1]) {
				i--
			}
			if i > 0 {
				message.seenBy = lines[i:]
			}
			// This is a silly heuristic.  Try to find origin and
			// tear lines within the last several lines of the
			// message.
			if i > 0 && strings.HasPrefix(lines[i-1], " * Origin:") {
				message.originLine = lines[i-1]
				i--
			} else if i > 1 && strings.HasPrefix(lines[i-2], " * Origin:") {
				message.originLine = lines[i-2]
				//i -= 2
			} else if i > 2 && strings.HasPrefix(lines[i-3], " * Origin:") {
				message.originLine = lines[i-3]
				//i -= 3
			}
			if i > 0 && strings.HasPrefix(lines[i-1], "---") {
				message.tearLine = lines[i-1]
				i--
			} else if i > 1 && strings.HasPrefix(lines[i-2], "---") {
				message.tearLine = lines[i-2]
				//i -= 2
			} else if i > 2 && strings.HasPrefix(lines[i-3], "---") {
				message.tearLine = lines[i-3]
				//i -= 3
			}
			message.body = lines[:i]

			message.Print()
		}
		fmt.Println("REST OF PACKET FILE:")
		_, err = io.Copy(os.Stdout, pr)
		if err != nil {
			log.Fatal("error copy:", err)
		}
		rc.Close()
		fmt.Println("END OF PACKET FILE:")
		fmt.Println()
	}
}

func isSeenBy(line string) bool {
	tokens := strings.SplitN(line, ":", 2)
	if len(tokens) != 2 {
		return false
	}
	return strings.ToLower(strings.TrimSpace(tokens[0])) == "seen-by"
}

func splitPacketLine(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
