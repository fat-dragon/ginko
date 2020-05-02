package ftn

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Zone int
type Net int
type Node int
type Point int
type Domain string

// A "5-dimensional" FTN address
type Address struct {
	zone   Zone
	net    Net
	node   Node
	point  Point
	domain Domain
}

// Returns an empty Address.
func emptyAddress() Address {
	return Address{0, 0, 0, 0, ""}
}

// Creates a new "5-Dimensional" address from the given data.
func NewAddress(zone Zone, net Net, node Node, point Point, domain Domain) Address {
	return Address{zone, net, node, point, domain}
}

// Creates a new "4-Dimensional" address from the given data.
func NewAddress4d(zone Zone, net Net, node Node, point Point) Address {
	return Address{zone, net, node, point, ""}
}

// Creates a new "2-Dimensional" address from the given data.
func NewAddress3d(zone Zone, net Net, node Node) Address {
	return Address{zone, net, node, 0, ""}
}

// Creates a new "2-Dimensional" address from the given data.
func NewAddress2d(net Net, node Node) Address {
	return Address{0, net, node, 0, ""}
}

// Returns a string containing the "5-dimensional"
// representation of an FTN address.  If the point,
// domain, or zone portions are missing, they are
// omitted.
func (a Address) String() string {
	var b strings.Builder
	b.WriteString(a.String4d())
	if a.domain != "" {
		b.WriteString("@")
		b.WriteString(string(a.domain))
	}
	return b.String()
}

// Returns a string containing the "5-dimensional"
// representation of an FTN address, with the
// point portion unconditionally included.  If
// domain is not present, it will not be
// included.
func (a Address) String5dPoint() string {
	var b strings.Builder
	b.WriteString(a.String4dPoint())
	if a.domain != "" {
		b.WriteString("@")
		b.WriteString(string(a.domain))
	}
	return b.String()
}

// Returns a string containing the "4-dimensional"
// representation of an FTN address.  If the point
// portion is missing, it is omitted and the
// result is the same as that of String3d().
func (a Address) String4d() string {
	var b strings.Builder
	b.WriteString(a.String3d())
	if a.point != 0 {
		fmt.Fprintf(&b, ".%d", a.point)
	}
	return b.String()
}

// Returns a string containing the "4-dimensional"
// representation of an FTN address, with the
// point portion unconditionally included.
func (a Address) String4dPoint() string {
	var b strings.Builder
	b.WriteString(a.String3d())
	fmt.Fprintf(&b, ".%d", a.point)
	return b.String()
}

// Returns a string containing the "3-dimensional"
// representation of an FTN address.  If the zone
// portion is missing, the same result is the same
// as that of String2d().
func (a Address) String3d() string {
	var b strings.Builder
	if a.zone != 0 {
		fmt.Fprintf(&b, "%d:", a.zone)
	}
	b.WriteString(a.String2d())
	return b.String()
}

// Returns a string containing the "2-dimensional"
// representation of an FTN address.
func (a Address) String2d() string {
	return fmt.Sprintf("%d/%d", a.net, a.node)
}

// Parses the string representation of an address and
// returns the resulting Address object.  Returns an
// empty address and an error if the address is
// malformed.
//
// Accepts a number of valid syntaxes:
//
//  zone:net/node.point@domain  (5d)
//  zone:net/node@domain        (5d, no point)
//  zone:net/nodd.point         (4d)
//  zone:net/node               (3d)
//  net/node                    (2d)
//
func ParseAddress(address string) (Address, error) {
	addr := strings.TrimSpace(address)
	zoneStr, addr1, hasZone := splitOn(addr, ":")
	if !hasZone {
		addr1 = zoneStr
	}
	netStr, addr2, _ := splitOn(addr1, "/")
	nodeStr, pointStr, hasPoint := splitOn(addr2, ".")
	domain := ""
	hasDomain := false
	if !hasPoint {
		nodeStr, domain, hasDomain = splitOn(nodeStr, "@")
	} else {
		pointStr, domain, hasDomain = splitOn(pointStr, "@")
	}

	zone := 0
	var err error
	if hasZone {
		zone, err = strconv.Atoi(zoneStr)
		if err != nil {
			return emptyAddress(), fmt.Errorf("invalid zone in %q: %v", address, err)
		}
	}

	net, err := strconv.Atoi(netStr)
	if err != nil {
		return emptyAddress(), fmt.Errorf("invalid net in %q: %v", address, err)
	}

	node, err := strconv.Atoi(nodeStr)
	if err != nil {
		return emptyAddress(), fmt.Errorf("invalid node in %q: %v", address, err)
	}

	point := 0
	if hasPoint {
		point, err = strconv.Atoi(pointStr)
		if err != nil {
			return emptyAddress(), fmt.Errorf("invalid point in %q: %v", address, err)
		}
	}

	if hasDomain {
		if domain == "" {
			return emptyAddress(), fmt.Errorf("invalid domain in %q", address)
		}
	}

	return Address{Zone(zone), Net(net), Node(node), Point(point), Domain(domain)}, nil
}

// Splits a string on `sep`.  Returns the token before `sep`,
// the rest of the string after `sep`, and a boolean indicating
// whether `sep` occurred.  If `sep` does not appear
// occur in the input, then the source is returned as the
// token and  the remainder value is the empty string.
// The boolean return value can help detect whether the
// separator the suffix was empty or the separate was
// missing.
func splitOn(s string, sep string) (string, string, bool) {
	token := s
	rest := ""
	found := false
	index := strings.Index(s, sep)
	if index > -1 {
		token = s[:index]
		rest = s[index+1:]
		found = true
	}
	return token, rest, found
}

// Unmarshal an FTN address from a string in a JSON stream.  We
// parse the address from the string and return that.
func (a *Address) UnmarshalJSON(data []byte) error {
	var addrStr string
	if err := json.Unmarshal(data, &addrStr); err != nil {
		return err
	}
	addr, err := ParseAddress(addrStr)
	if err != nil {
		return err
	}
	*a = addr
	return nil
}
