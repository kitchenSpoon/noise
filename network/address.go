package network

import (
	"net"
	"net/url"
	"strconv"
)

// AddressInfo represents a network URL.
type AddressInfo struct {
	Protocol string
	Host     string
	Port     uint16
}

// NewAddressInfo creates a new address info instance.
func NewAddressInfo(protocol, host string, port uint16) *AddressInfo {
	return &AddressInfo{
		Protocol: protocol,
		Host:     host,
		Port:     port,
	}
}

// String prints out either the URL representation of the address info, or
// solely just a joined host and port should a network scheme not be defined.
func (info *AddressInfo) String() string {
	address := net.JoinHostPort(info.Host, strconv.Itoa(int(info.Port)))
	if len(info.Protocol) > 0 {
		address = info.Protocol + "://" + address
	}
	return address
}

// Raw returns the string representation of host:port.
func (info *AddressInfo) Raw() string {
	return net.JoinHostPort(info.Host, strconv.Itoa(int(info.Port)))
}

// ParseAddress derives a network scheme, host and port of a destinations
// information. Errors should the provided destination address be malformed.
func ParseAddress(address string) (*AddressInfo, error) {
	urlInfo, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	host, rawPort, err := net.SplitHostPort(urlInfo.Host)
	if err != nil {
		return nil, err
	}

	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil {
		return nil, err
	}

	return &AddressInfo{
		Protocol: urlInfo.Scheme,
		Host:     host,
		Port:     uint16(port),
	}, nil
}

// FormatAddress properly marshals a destinations information into a string.
func FormatAddress(protocol, host string, port uint16) string {
	return NewAddressInfo(protocol, host, port).String()
}
