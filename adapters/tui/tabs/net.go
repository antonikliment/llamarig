package tabs

import (
	"net"
	"strings"
)

func publicBaseURL(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" || host == "-" {
		host = "127.0.0.1"
	}
	if port == "" {
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		return "http://" + host
	}
	return "http://" + net.JoinHostPort(host, port)
}
