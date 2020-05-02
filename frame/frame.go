package frame

import (
	"fmt"
	"io"
	"strings"
	"time"

	"fat-dragon.org/ginko/ftn"
)

// Data contains bytes of uninterpreted data,
// usually file contents.
type Data struct {
	data []byte
}

// Data returns the data held by the frame.
func (d *Data) Data() []byte {
	return d.data
}

func (d *Data) Length() int {
	return len(d.data)
}

func (d *Data) String() string {
	const MaxStr = 10
	if d.Length() > MaxStr {
		return fmt.Sprintf("DATA: [%d]byte(%.*q...)", len(d.data), MaxStr, d.data)
	}
	return fmt.Sprintf("DATA: [%d]byte(%q)", len(d.data), d.data)
}

// NullCmd mostly contains information for humans,
// The exception is that it may contain options, which
// influence the operation of the protocol.  These are
// special, so we distinguish them with a dedicated type
// (see `OptCmd`, below).
type NullCmd struct {
	text string
}

func (c *NullCmd) String() string {
	return c.text
}

// NewNull creates a new NUL command frame with the specified
// text.
func NewNull(text string) *NullCmd {
	return &NullCmd{text}
}

// OptCmd is actually a special case of an M_NUL frame.
// Options are special and important to the protocol, so even
// though the protocol does not define a special message for
// them, we treat them specially.
type OptCmd struct {
	options []string
}

func (c *OptCmd) String() string {
	return "OPT: [" + strings.Join(c.options, ", ") + "]"
}

// NewOpt creates a new OptCmd frame with the given options.
func NewOpt(options ...string) *OptCmd {
	return &OptCmd{options}
}

// Options returns the OptCmd's options.
func (c *OptCmd) Options() []string {
	return c.options
}

// NewChallenge creates an OptCmd frame with the given
// challenge in the appropriate format.
func NewChallenge(hash, value string) *OptCmd {
	return &OptCmd{[]string{fmt.Sprintf("CRAM-%s-%s", hash, value)}}
}

// AddressCmd contains a list of addresses that the
// distant end speaks for.
type AddressCmd struct {
	addrs []ftn.Address
}

func (c *AddressCmd) String() string {
	var b strings.Builder
	b.WriteString("ADR: [")
	sep := ""
	for _, addr := range c.addrs {
		b.WriteString(sep)
		b.WriteString(addr.String())
		sep = ", "
	}
	b.WriteString("]")
	return b.String()
}

// NewAddress creates an AddressCmd populated with the given
// addresses.
func NewAddress(addresses ...ftn.Address) *AddressCmd {
	return &AddressCmd{addresses}
}

// Addresses returns the slice of addresses contained in the
// AddressCmd.
func (c *AddressCmd) Addresses() []ftn.Address {
	return c.addrs
}

// PasswdCmd contains either a password or a response
// to an earlier CRAM challenge.  The text is exposed
// so that we can access it from the protocol driver
// for validation (if receiver).
type PasswdCmd struct {
	Password string
}

func (c *PasswdCmd) String() string {
	if strings.HasPrefix(c.Password, "CRAM-") {
		return "PWD: " + c.Password
	}
	return "PWD: (password)"
}

// NewPassword creates a PasswdCmd with the given (plaintext)
// password.
func NewPassword(passwd string) *PasswdCmd {
	return &PasswdCmd{passwd}
}

// NewResponse creates a PasswdCmd with the given CRAM response
// and hash algorithm.
func NewResponse(hash, value string) *PasswdCmd {
	return &PasswdCmd{fmt.Sprintf("CRAM-%s-%s", hash, value)}
}

// FileCmd contains information about a file that the
// distant end is proposing to transfer to us.
type FileCmd struct {
	FileName  string
	Size      int64
	TimeStamp time.Time
	Offset    int64
	CRC       uint32
	ExtraOpts []string
}

func (c *FileCmd) String() string {
	return fmt.Sprintf("FILE: %q size %d timestamp \"%v\" offset %d crc %x flags %v",
		c.FileName, c.Size, c.TimeStamp, c.Offset, c.CRC, c.ExtraOpts)
}

// NewFileCmd returns a new file command frame.
func NewFileCmd(fileName string, size int64, timeStamp time.Time, offset int64) *FileCmd {
	return &FileCmd{fileName, size, timeStamp, offset, 0, nil}
}

// OkCmd indicates successful authentication of the distant
// end.
type OkCmd struct {
	text string
}

func (c *OkCmd) String() string {
	return "OK " + c.text
}

// NewOk creates a new OkCmd with the given text.
func NewOk(text string) *OkCmd {
	return &OkCmd{text}
}

// EOBCmd indicates the end of a transfer of a batch of files.
type EOBCmd struct{}

func (c *EOBCmd) String() string {
	return "EOB"
}

// NewEOB creates a new EOBCmd.
func NewEOB() *EOBCmd {
	return &EOBCmd{}
}

// GotCmd acknowledges successful receipt of a file transmitted
// from the distant end.
type GotCmd struct {
	FileName  string
	Size      int64
	TimeStamp time.Time
}

func (c *GotCmd) String() string {
	return fmt.Sprintf("GOT file %q, size %v, timestamp %v",
		c.FileName, c.Size, c.TimeStamp)
}

// NewGot returns a new GotCmd with the given transfer parameters.
func NewGot(fileName string, size int64, timeStamp time.Time) *GotCmd {
	return &GotCmd{fileName, size, timeStamp}
}

// ErrorCmd represents some kind of fatal error.  The included
// should indicate the nature of the error.
type ErrorCmd struct {
	text string
}

func (c *ErrorCmd) String() string {
	return "ERROR " + c.text
}

// NewErrorCmd returns a new ErrorCmd with the given text.
// Unlike other "New" functions in this package, the name
// includes the "Cmd" suffix to distinguish its purpose
// from something having to do with the built-in error type.
func NewErrorCmd(text string) *ErrorCmd {
	return &ErrorCmd{text}
}

// BusyCmd indicates a busy status on the distant end.  It
// is transient.
type BusyCmd struct {
	text string
}

func (c *BusyCmd) String() string {
	return "BUSY " + c.text
}

// GetCmd represnts a request for a file transfer from the
// distant end.
type GetCmd struct {
	FileName  string
	Size      int64
	TimeStamp time.Time
	Offset    int64
}

func (c *GetCmd) String() string {
	return fmt.Sprintf("GET file %q, size %v, timestamp %v, offset %v",
		c.FileName, c.Size, c.TimeStamp, c.Offset)
}

// SkipCmd is a temporary request to the distant end to skip
// sending a particular file.
type SkipCmd struct {
	FileName  string
	Size      int64
	TimeStamp time.Time
}

func (c *SkipCmd) String() string {
	return fmt.Sprintf("SKIP file %q, size %v, timeStamp %v",
		c.FileName, c.Size, c.TimeStamp)
}

// Command is a marker interface for a command frames.
type Command interface {
	isCommand()
}

func (c NullCmd) isCommand()    {}
func (c OptCmd) isCommand()     {}
func (c AddressCmd) isCommand() {}
func (c PasswdCmd) isCommand()  {}
func (c FileCmd) isCommand()    {}
func (c OkCmd) isCommand()      {}
func (c EOBCmd) isCommand()     {}
func (c GotCmd) isCommand()     {}
func (c ErrorCmd) isCommand()   {}
func (c BusyCmd) isCommand()    {}
func (c GetCmd) isCommand()     {}
func (c SkipCmd) isCommand()    {}

// Frame is a marker interface for all frame types.
type Frame interface {
	isFrame()
	WriteBytes(out io.Writer) error
}

func (c NullCmd) isFrame()    {}
func (c OptCmd) isFrame()     {}
func (c AddressCmd) isFrame() {}
func (c PasswdCmd) isFrame()  {}
func (c FileCmd) isFrame()    {}
func (c OkCmd) isFrame()      {}
func (c EOBCmd) isFrame()     {}
func (c GotCmd) isFrame()     {}
func (c ErrorCmd) isFrame()   {}
func (c BusyCmd) isFrame()    {}
func (c GetCmd) isFrame()     {}
func (c SkipCmd) isFrame()    {}
func (d Data) isFrame()       {}

// Terminal is an interface for frame types
// that terminate a session.
type Terminal interface {
	isTerminal()
	WriteBytes(out io.Writer) error
}

func (c ErrorCmd) isTerminal() {}
func (c BusyCmd) isTerminal()  {}

// ChangesQueue is a marker interface for frames types
// that change the transmission queue.
type Queueing interface {
	isQueueing()
}

func (c GetCmd) isQueueing()  {}
func (c GotCmd) isQueueing()  {}
func (c SkipCmd) isQueueing() {}
