package ftn

import "testing"

func TestAddressString5dNoPoint(t *testing.T) {
	stringTest(NewAddress(21, 100, 198, 0, "fsxnet"), "21:100/198@fsxnet", t)
}

func TestAddressString5d(t *testing.T) {
	stringTest(NewAddress(21, 100, 198, 1, "fsxnet"), "21:100/198.1@fsxnet", t)
}

func TestAddressString4d(t *testing.T) {
	stringTest(NewAddress4d(21, 100, 198, 1), "21:100/198.1", t)
}

func TestAddressString3d(t *testing.T) {
	stringTest(NewAddress3d(21, 100, 198), "21:100/198", t)
}

func TestAddressString2d(t *testing.T) {
	stringTest(NewAddress2d(100, 198), "100/198", t)
}

func stringTest(address Address, expected string, t *testing.T) {
	s := address.String()
	if s != expected {
		t.Errorf("Bad address string, got %q expected %q", s, expected)
	}
}

func TestParse5dAddress(t *testing.T) {
	parseTest("21:100/198.1@fsxnet", "21:100/198.1@fsxnet", t)
	parseTest("21:100/198.0@fsxnet", "21:100/198@fsxnet", t)
	parseTest("21:100/198@fsxnet", "21:100/198@fsxnet", t)
	parseTest("21:1/100@fsxnet", "21:1/100@fsxnet", t)
	parseTest("21:1/3@fsxnet", "21:1/3@fsxnet", t)
	parseTest("21:1/2@fsxnet", "21:1/2@fsxnet", t)
	parseTest("21:1/0@fsxnet", "21:1/0@fsxnet", t)
	parseTest("21:0/0@fsxnet", "21:0/0@fsxnet", t)
}

func TestParse4dAddress(t *testing.T) {
	parseTest("21:100/198.1", "21:100/198.1", t)
	parseTest("21:100/198.0", "21:100/198", t)
}

func TestParse3dAddress(t *testing.T) {
	parseTest("21:100/198", "21:100/198", t)
}

func TestParse2dAddress(t *testing.T) {
	parseTest("100/198", "100/198", t)
}

func parseTest(addr string, expected string, t *testing.T) {
	address, err := ParseAddress(addr)
	if err != nil {
		t.Errorf("Parsing %q failed: %v", addr, err)
	}
	s := address.String()
	if s != expected {
		t.Errorf("Expected parse result %q got %q", expected, s)
	}
}

func TestParseInvalidZone(t *testing.T) {
	parseInvalidTest(":100/198@fsxnet", t)
	parseInvalidTest(":100/198.1@fsxnet", t)
	parseInvalidTest("aaa:100/198@fsxnet", t)
}

func TestParseInvalidNet(t *testing.T) {
	parseInvalidTest("21:/198@fsxnet", t)
	parseInvalidTest("21:aaa/198", t)
	parseInvalidTest("aaa/198", t)
}

func TestParseInvalidNode(t *testing.T) {
	parseInvalidTest("21:100/", t)
	parseInvalidTest("21:100/.0@fsxnet", t)
	parseInvalidTest("21:100/.0", t)
	parseInvalidTest("21:100/.1", t)
	parseInvalidTest("21:100/aaa.0", t)
	parseInvalidTest("21:100/aaa.1", t)
	parseInvalidTest("100/", t)
	parseInvalidTest("100/.123", t)
}

func TestParseInvalidPoint(t *testing.T) {
	parseInvalidTest("21:100/198.", t)
	parseInvalidTest("21:100/198.aaa", t)
	parseInvalidTest("21:100/198.@fsxnet", t)
	parseInvalidTest("21:100/198.aaa@fsxnet", t)
	parseInvalidTest("100/198.aaa@fsxnet", t)
}

func TestParseInvalidNetHostPermutations(t *testing.T) {
	parseInvalidTest("21:.0", t)
	parseInvalidTest("21:100.0", t)
	parseInvalidTest("21:100@fsxnet", t)
	parseInvalidTest("21:@fsxnet", t)
	parseInvalidTest("21:aaa@fsxnet", t)
}

func parseInvalidTest(addr string, t *testing.T) {
	address, err := ParseAddress(addr)
	if err == nil {
		t.Errorf("Unexpected success parsing %q got %q", addr, address.String())
	}
}
