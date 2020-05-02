package frame

import (
	"bytes"
	"testing"
)

const (
	cmdFr  = 0x80
	dataFr = 0x00
)

func TestReadDataFrame(t *testing.T) {
	data := "Hello"
	header := []byte{dataFr, byte(len(data))}
	frameBytes := append(header, []byte(data)...)
	buffer := bytes.NewBuffer(frameBytes)
	frame, err := Read(buffer)
	if err != nil {
		t.Error("invalid frame:", err)
	}
	dataFrame, ok := frame.(*Data)
	if !ok {
		t.Error("Frame is not Data")
	}
	if string(dataFrame.Data()) != data {
		t.Errorf("Bad frame data: %q", dataFrame.data)
	}
}

func TestReadNullCmdFrame(t *testing.T) {
	data := "Hello"
	header := []byte{cmdFr, byte(len(data) + 1)}
	frameBytes := append(append(header, CmdNUL), []byte(data)...)
	buffer := bytes.NewBuffer(frameBytes)
	frame, err := Read(buffer)
	if err != nil {
		t.Error("invalid frame:", err)
	}
	nullCmd, ok := frame.(*NullCmd)
	if !ok {
		t.Error("Frame is not NullCmd")
	}
	if nullCmd.text != "Hello" {
		t.Errorf("Bad frame data: %q", nullCmd.text)
	}
}

func TestReadAddressCmdFrame(t *testing.T) {
	expected := "ADR: [21:1/100@fsxnet, 21:1/3@fsxnet, 21:1/2@fsxnet, 21:1/0@fsxnet, 21:0/0@fsxnet]"
	data := "  21:1/100@fsxnet 21:1/3@fsxnet   21:1/2@fsxnet 21:1/0@fsxnet 21:0/0@fsxnet  \x00\x00"
	header := []byte{cmdFr, byte(len(data) + 1)}
	frameBytes := append(append(header, CmdADR), []byte(data)...)
	buffer := bytes.NewBuffer(frameBytes)
	frame, err := Read(buffer)
	if err != nil {
		t.Error("invalid frame:", err)
	}
	addrCmd, ok := frame.(*AddressCmd)
	if !ok {
		t.Error("Frame is not AddressCmd")
	}
	acStr := addrCmd.String()
	if acStr != expected {
		t.Errorf("Address parse expected %q got %q", expected, acStr)
	}
}

func TestReadPasswdCmd(t *testing.T) {
	expected := "PWD: (password)"
	data := "THISISNOTMYPASSWORD"
	header := []byte{cmdFr, byte(len(data) + 1)}
	frameBytes := append(append(header, CmdPWD), []byte(data)...)
	buffer := bytes.NewBuffer(frameBytes)
	frame, err := Read(buffer)
	if err != nil {
		t.Error("invalid frame:", err)
	}
	passwdCmd, ok := frame.(*PasswdCmd)
	if !ok {
		t.Error("Frame is not PasswdCmd")
	}
	pcStr := passwdCmd.String()
	if pcStr != expected {
		t.Errorf("Password parse expected %q got %q", expected, pcStr)
	}
}

func TestReadCRAMPasswdCmd(t *testing.T) {
	expected := "PWD: CRAM-md5-deadcafe"
	data := "CRAM-md5-deadcafe"
	header := []byte{cmdFr, byte(len(data) + 1)}
	frameBytes := append(append(header, CmdPWD), []byte(data)...)
	buffer := bytes.NewBuffer(frameBytes)
	frame, err := Read(buffer)
	if err != nil {
		t.Error("invalid frame:", err)
	}
	passwdCmd, ok := frame.(*PasswdCmd)
	if !ok {
		t.Error("Frame is not PasswdCmd")
	}
	pcStr := passwdCmd.String()
	if pcStr != expected {
		t.Errorf("Password parse expected %q got %q", expected, pcStr)
	}
}

func TestReadFileCmd(t *testing.T) {
	expected := `FILE: "foo.txt" size 1234 timestamp "1970-01-01 00:01:40 +0000 UTC" offset 2 crc 0 flags []`
	data := "foo.txt 1234 100 2"
	header := []byte{cmdFr, byte(len(data) + 1)}
	frameBytes := append(append(header, CmdFILE), []byte(data)...)
	buffer := bytes.NewBuffer(frameBytes)
	frame, err := Read(buffer)
	if err != nil {
		t.Error("invalid frame:", err)
	}
	fileCmd, ok := frame.(*FileCmd)
	if !ok {
		t.Error("Frame is not FileCmd")
	}
	fcStr := fileCmd.String()
	if fcStr != expected {
		t.Errorf("File parse expected %q got %q", expected, fcStr)
	}
}

func TestReadFileCmdCRCOpt(t *testing.T) {
	expected := `FILE: "foo.txt" size 1234 timestamp "1970-01-01 00:01:40 +0000 UTC" offset 2 crc cafef00d flags []`
	data := "foo.txt 1234 100 2 cafef00d"
	header := []byte{cmdFr, byte(len(data) + 1)}
	frameBytes := append(append(header, CmdFILE), []byte(data)...)
	buffer := bytes.NewBuffer(frameBytes)
	frame, err := Read(buffer)
	if err != nil {
		t.Error("invalid frame:", err)
	}
	fileCmd, ok := frame.(*FileCmd)
	if !ok {
		t.Error("Frame is not FileCmd")
	}
	fcStr := fileCmd.String()
	if fcStr != expected {
		t.Errorf("File parse expected %q got %q", expected, fcStr)
	}
}

func TestReadFileCmdExtraOpt(t *testing.T) {
	expected := `FILE: "foo.txt" size 1234 timestamp "1970-01-01 00:01:40 +0000 UTC" offset 2 crc 0 flags [BZ2]`
	data := "foo.txt 1234 100 2 BZ2"
	header := []byte{cmdFr, byte(len(data) + 1)}
	frameBytes := append(append(header, CmdFILE), []byte(data)...)
	buffer := bytes.NewBuffer(frameBytes)
	frame, err := Read(buffer)
	if err != nil {
		t.Error("invalid frame:", err)
	}
	fileCmd, ok := frame.(*FileCmd)
	if !ok {
		t.Error("Frame is not FileCmd")
	}
	fcStr := fileCmd.String()
	if fcStr != expected {
		t.Errorf("File parse expected %q got %q", expected, fcStr)
	}
}

func TestReadFileCmdCRCAndExtraOpt(t *testing.T) {
	expected := `FILE: "foo.txt" size 1234 timestamp "1970-01-01 00:01:40 +0000 UTC" offset 2 crc deadf00d flags [BZ2]`
	data := "foo.txt 1234 100 2 deadf00d BZ2"
	header := []byte{cmdFr, byte(len(data) + 1)}
	frameBytes := append(append(header, CmdFILE), []byte(data)...)
	buffer := bytes.NewBuffer(frameBytes)
	frame, err := Read(buffer)
	if err != nil {
		t.Error("invalid frame:", err)
	}
	fileCmd, ok := frame.(*FileCmd)
	if !ok {
		t.Error("Frame is not FileCmd")
	}
	fcStr := fileCmd.String()
	if fcStr != expected {
		t.Errorf("File parse expected %q got %q", expected, fcStr)
	}
}

func TestReadInvalidEmptyDataFrame(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{dataFr, 0})
	frame, err := Read(buffer)
	if frame != nil || err == nil {
		t.Error("Invalid empty data frame accepted")
	}
}

func TestReadInvalidEmptyCmdFrame(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{cmdFr, 0})
	frame, err := Read(buffer)
	if frame != nil || err == nil {
		t.Error("Invalid empty command frame accepted")
	}
}

func TestReadInvalidCommands(t *testing.T) {
	for i := int(maxCmds); i < 256; i++ {
		buffer := bytes.NewBuffer([]byte{cmdFr, 1, byte(i)})
		frame, err := Read(buffer)
		if frame != nil || err == nil {
			t.Error("Invalid frame type read frame")
		}
	}
}
