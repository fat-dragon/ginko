package frame

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"fat-dragon.org/ginko/ftn"
)

// Protocol-defined constants for frame types.
const (
	CmdNUL byte = iota
	CmdADR
	CmdPWD
	CmdFILE
	CmdOK
	CmdEOB
	CmdGOT
	CmdERR
	CmdBSY
	CmdGET
	CmdSKIP
	maxCmds // For testing.
)

const MaxFrameSize = 32767

// Read takes raw frame bytes from `reader` and returns a decoded
// Frame structure.
func Read(reader io.Reader) (Frame, error) {
	var header [2]byte
	hnb, err := io.ReadFull(reader, header[:])
	if err != nil {
		return nil, err
	}
	if hnb != len(header) {
		return nil, fmt.Errorf("short header read (%v of %v): %v", hnb, len(header), err)
	}
	length := int(header[0]&0x7F)<<8 | int(header[1])
	if length == 0 {
		return nil, errors.New("invalid empty frame")
	}
	data := make([]byte, length, length)
	dnb, err := io.ReadFull(reader, data)
	if err != nil {
		return nil, err
	}
	if dnb != length {
		return nil, fmt.Errorf("short data read (%v of %v): %v", dnb, length, err)
	}
	return decodeFrame(header, data)
}

// Actually decodes frames.  The frame type is inspected and a
// structure of the appropriate type is created and returned.
func decodeFrame(header [2]byte, data []byte) (Frame, error) {
	const frDATA = 0
	frameType := header[0] >> 7
	if frameType == frDATA {
		return &Data{data}, nil
	}
	commandType := data[0]
	data = data[1:]
	switch commandType {
	case CmdNUL:
		return decodeNullCmd(data)
	case CmdADR:
		return decodeAddressCmd(data)
	case CmdPWD:
		return &PasswdCmd{dataToString(data)}, nil
	case CmdFILE:
		return decodeFileCmd(data)
	case CmdOK:
		return &OkCmd{dataToString(data)}, nil
	case CmdEOB:
		return &EOBCmd{}, nil
	case CmdGOT:
		return decodeGotCmd(data)
	case CmdERR:
		return &ErrorCmd{dataToString(data)}, nil
	case CmdBSY:
		return &BusyCmd{dataToString(data)}, nil
	case CmdGET:
		return &GetCmd{}, nil
	case CmdSKIP:
		return &SkipCmd{}, nil
	}
	return nil, fmt.Errorf("invalid command frame type %v", commandType)
}

// Decodes an M_NUL command.  Takes special care to detect
// an "OPT" packet and return it specially.
func decodeNullCmd(data []byte) (Frame, error) {
	text := dataToString(data)
	if strings.HasPrefix(text, "OPT ") {
		return &OptCmd{strings.Fields(text)[1:]}, nil
	}
	return &NullCmd{text}, nil
}

// Decodes an M_ADR command.  Each specified address is parsed into
// an internal representation,
func decodeAddressCmd(data []byte) (Frame, error) {
	s := dataToString(data)
	addrStrs := strings.Fields(s)
	addrs := make([]ftn.Address, len(addrStrs), len(addrStrs))
	for i, addrStr := range addrStrs {
		addr, err := ftn.ParseAddress(addrStr)
		if err != nil {
			return nil, err
		}
		addrs[i] = addr
	}
	return &AddressCmd{addrs}, nil
}

// Decodes an M_FILE command.  Note that, depending on extensions,
// the M_FILE command may have extra parameters.
func decodeFileCmd(data []byte) (Frame, error) {
	fileName, size, timeStamp, offset, crc, extraOpts, err := decodeFileParams(data)
	if err != nil {
		return nil, fmt.Errorf("FILE decode failed: %v", err)
	}
	return &FileCmd{fileName, size, timeStamp, offset, crc, extraOpts}, nil
}

// Decodes an M_GOT command.
func decodeGotCmd(data []byte) (Frame, error) {
	fileName, size, timeStamp, err := decodeFileIDParams(data)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode GOT: %v", err)
	}
	return &GotCmd{fileName, size, timeStamp}, nil
}

// Decodes an M_GET command.
func decodeGetCmd(data []byte) (Frame, error) {
	fileName, size, timeStamp, offset, err := decodeFileTransferParams(data)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode GET: %v", err)
	}
	return &GetCmd{fileName, size, timeStamp, offset}, nil
}

// Decodes an M_SKIP command.
func decodeSkipCmd(data []byte) (Frame, error) {
	fileName, size, timeStamp, err := decodeFileIDParams(data)
	if err != nil {
		return nil, fmt.Errorf("SKIP decode failed: %v", err)
	}
	return &SkipCmd{fileName, size, timeStamp}, nil
}

// Decodes all file parameters, including optional CRC and other
// arguments.
func decodeFileParams(data []byte) (fileName string, size int64, timeStamp time.Time, offset int64, crc uint32, extraOpts []string, err error) {
	s := dataToString(data)
	fields := strings.Fields(s)
	fileName, size, timeStamp, offset, err = decodeFileTransferParamFields(s, fields[:min(4, len(fields))])
	if err != nil {
		err = fmt.Errorf("decodeFileParams: %v", err)
		return
	}
	for _, opt := range fields[4:] {
		maybeCrc, crcErr := strconv.ParseUint(opt, 16, 32)
		if crcErr == nil {
			crc = uint32(maybeCrc)
		} else {
			extraOpts = append(extraOpts, opt)
		}
	}
	return
}

// Decodes file transfer parameters.  These are the file ID parameters
// (name, size, time stamp) as well as an offset.
func decodeFileTransferParams(data []byte) (fileName string, size int64, timeStamp time.Time, offset int64, err error) {
	s := dataToString(data)
	fields := strings.Fields(s)
	fileName, size, timeStamp, offset, err = decodeFileTransferParamFields(s, fields)
	if err != nil {
		err = fmt.Errorf("decodeFileTransferParams: %v", err)
	}
	return
}

// Decodes file transfer parameters that have already been extracted into fields.
func decodeFileTransferParamFields(s string, fields []string) (fileName string, size int64, timeStamp time.Time, offset int64, err error) {
	if len(fields) != 4 {
		err = fmt.Errorf("%q has wrong number of fields", s)
		return
	}
	fileName, size, timeStamp, err = decodeFileIDParamFields(s, fields[:3])
	if err != nil {
		return
	}
	offset, err = strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		err = fmt.Errorf("offset %q in %q error: %v", fields[3], s, err)
	}
	return
}

// Decodes basic file identification paramters: (name, size, time stamp).
func decodeFileIDParams(data []byte) (fileName string, size int64, timeStamp time.Time, err error) {
	s := dataToString(data)
	fields := strings.Fields(s)
	fileName, size, timeStamp, err = decodeFileIDParamFields(s, fields)
	if err != nil {
		err = fmt.Errorf("decodeFileIDParams: %v", err)
	}
	return
}

// Decodes basic file identification paramters that have already been extracted
// into fields.
func decodeFileIDParamFields(s string, fields []string) (fileName string, size int64, timeStamp time.Time, err error) {
	if len(fields) != 3 {
		err = fmt.Errorf("%q has wrong number of fields", s)
		return
	}
	fileName = fields[0]
	size, err = strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		err = fmt.Errorf("size %q in %q error: %v", fields[1], s, err)
		return
	}
	unixSecs, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		err = fmt.Errorf("time stamp %q in %q error: %v", fields[2], s, err)
		return
	}
	timeStamp = time.Unix(unixSecs, 0)
	return
}

// Converts a byte slice containing textual data from a command
// frame into a string.  Takes care to strip any NUL terminating
// characters that may be present from the input.
func dataToString(data []byte) string {
	s := string(data)
	s = strings.TrimRight(s, "\x00")
	return strings.TrimSpace(s)
}

// WriteBytes encodes a data frame into a writer.
func (d *Data) WriteBytes(out io.Writer) error {
	header, err := encodeDataHeader(len(d.data))
	if err != nil {
		return err
	}
	hb, err := out.Write(header[:])
	if err != nil {
		return fmt.Errorf("header write failed, wrote %d bytes: %v", hb, err)
	}
	db, err := out.Write(d.data)
	if err != nil {
		return fmt.Errorf("data write failed, wrote %d bytes: %v", db, err)
	}
	return nil
}

// ReadDataFrameFrom reads data from a file and copies it into a Data frame.
func ReadDataFrameFrom(in io.Reader, offset int64) (*Data, error) {
	b := bytes.NewBuffer([]byte{})
	_, err := io.CopyN(b, in, MaxFrameSize)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return &Data{b.Bytes()}, nil
}

var emptyHeader = [2]byte{0, 0}

func encodeDataHeader(length int) ([2]byte, error) {
	if length > MaxFrameSize {
		return emptyHeader, fmt.Errorf("Data frame size %d too long", length)
	}
	return [2]byte{byte(length >> 8), byte(length)}, nil
}

func encodeCmdHeader(length int) ([2]byte, error) {
	if length > MaxFrameSize {
		return emptyHeader, fmt.Errorf("Command frame size %d too long", length)
	}
	return [2]byte{byte(length>>8) | 0x80, byte(length)}, nil
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// WriteBytes (c *NulCmd) writes a NulCmd frame to an io.Writer,
// which may refer to a byte buffer, a socket, etc.
func (c *NullCmd) WriteBytes(out io.Writer) error {
	b := makeFrameBuffer()
	b.WriteByte(CmdNUL)
	b.WriteString(c.text)
	return copyCmdOut("NUL", b.Bytes(), out)
}

// WriteBytes (c *OptCmd) writes an OptCmd to an io.Wtier.
func (c *OptCmd) WriteBytes(out io.Writer) error {
	b := makeFrameBuffer()
	b.WriteByte(CmdNUL)
	b.WriteString("OPT")
	for _, option := range c.options {
		b.WriteString(" ")
		b.WriteString(option)
	}
	return copyCmdOut("OPT", b.Bytes(), out)
}

// WriteBytes (c *AddressCmd) writes an AddressCmd to an io.Writer.
func (c *AddressCmd) WriteBytes(out io.Writer) error {
	b := makeFrameBuffer()
	b.WriteByte(CmdADR)
	sep := ""
	for _, addr := range c.addrs {
		b.WriteString(sep)
		b.WriteString(addr.String())
		sep = " "
	}
	return copyCmdOut("ADR", b.Bytes(), out)
}

// WriteBytes (c *PasswdCmd) writes a PasswdCmd to an io.Writer.
func (c *PasswdCmd) WriteBytes(out io.Writer) error {
	b := makeFrameBuffer()
	b.WriteByte(CmdPWD)
	b.WriteString(c.Password)
	return copyCmdOut("PWD", b.Bytes(), out)
}

// WriteBytes (c *FileCmd) writes a FileCmd frame to an io.Writer.
func (c *FileCmd) WriteBytes(out io.Writer) error {
	b := makeFrameBuffer()
	b.WriteByte(CmdFILE)
	b.WriteString(c.FileName)
	b.WriteString(fmt.Sprintf(" %v %v %v", c.Size, c.TimeStamp.Unix(), c.Offset))
	if c.CRC != 0 {
		b.WriteString(fmt.Sprintf(" %v", c.CRC))
	} else {
		sep := ""
		for _, extraOpt := range c.ExtraOpts {
			b.WriteString(sep)
			b.WriteString(extraOpt)
			sep = " "
		}
	}
	return copyCmdOut("FILE", b.Bytes(), out)
}

// WriteBytes (c *OkCmd) writes an OkCmd frame to an io.Writer.
func (c *OkCmd) WriteBytes(out io.Writer) error {
	b := makeFrameBuffer()
	b.WriteByte(CmdOK)
	b.WriteString(c.text)
	return copyCmdOut("OK", b.Bytes(), out)
}

// WriteBytes (c *EOBCmd) writes an EOBCmd frame to an io.Writer.
func (c *EOBCmd) WriteBytes(out io.Writer) error {
	b := makeFrameBuffer()
	b.WriteByte(CmdEOB)
	return copyCmdOut("OK", b.Bytes(), out)
}

// WriteBytes (c *GotCmd) writes a GotCmd frame to an io.Writer.
func (c *GotCmd) WriteBytes(out io.Writer) error {
	b := makeFrameBuffer()
	b.WriteByte(CmdGOT)
	b.WriteString(c.FileName)
	b.WriteString(fmt.Sprintf(" %v %v", c.Size, c.TimeStamp.Unix()))
	return copyCmdOut("GOT", b.Bytes(), out)
}

// WriteBytes (c *ErrorCmd) writes an ErrorCmd frame to an io.Writer.
func (c *ErrorCmd) WriteBytes(out io.Writer) error {
	b := makeFrameBuffer()
	b.WriteByte(CmdERR)
	b.WriteString(c.text)
	return copyCmdOut("ERR", b.Bytes(), out)
}

// WriteBytes (c *BusyCmd) writes a BusyCmd frame to an io.Writer.
func (c *BusyCmd) WriteBytes(out io.Writer) error {
	b := makeFrameBuffer()
	b.WriteByte(CmdBSY)
	b.WriteString(c.text)
	return copyCmdOut("BSY", b.Bytes(), out)
}

// WriteBytes (c *GetCmd) writes a GetCmd frame to an io.Writer.
func (c *GetCmd) WriteBytes(out io.Writer) error {
	b := makeFrameBuffer()
	b.WriteByte(CmdGET)
	b.WriteString(c.FileName)
	b.WriteString(fmt.Sprintf(" %v %v %v", c.Size, c.TimeStamp.Unix(), c.Offset))
	return copyCmdOut("GET", b.Bytes(), out)
}

// WriteBytes (c *SkipCmd) writes a SkipCmd frame to an io.Writer.
func (c *SkipCmd) WriteBytes(out io.Writer) error {
	b := makeFrameBuffer()
	b.WriteByte(CmdSKIP)
	b.WriteString(c.FileName)
	b.WriteString(fmt.Sprintf(" %v %v", c.Size, c.TimeStamp.Unix()))
	return copyCmdOut("SKIP", b.Bytes(), out)
}

// makeFrameBuffer returns a new bytes.Buffer with space for
// a frame header at the front.
func makeFrameBuffer() bytes.Buffer {
	var b bytes.Buffer
	b.Write(emptyHeader[:])
	return b
}

// copyCmdOut creates a header for the given byte slice, writes
// it into the (reserved) beginning of that slice, and
func copyCmdOut(typ string, frameBytes []byte, out io.Writer) error {
	header, err := encodeCmdHeader(len(frameBytes) - 2)
	if err != nil {
		return fmt.Errorf("making "+typ+" header failed: %v", err)
	}
	copy(frameBytes[:2], header[:])
	nb, err := out.Write(frameBytes)
	if err != nil {
		return fmt.Errorf(typ+" cmd write failed, wrote %d bytes: %v", nb, err)
	}
	return nil
}
