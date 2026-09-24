package ip_helper

import (
	"fmt"
	"net"
	"testing"

	"go.uber.org/goleak"
)

func TestGetExternalIPV4(t *testing.T) {
	goleak.VerifyNone(t)

	ipv4 := make(chan string)
	go func() { ipv4 <- GetExternalIPV4() }()
	fmt.Println(<-ipv4)
}

func TestGetExternalIPV6(t *testing.T) {
	ipv6 := make(chan string)
	go func() { ipv6 <- GetExternalIPV6() }()
	fmt.Println(<-ipv6)
}

func TestGetLoclIp(t *testing.T) {
	fmt.Println(GetLoclIp())
}

func TestHasLocalIP(t *testing.T) {
	fmt.Println("dddd")
	fmt.Println(HasLocalIP(net.ParseIP("192.168.2.10")))
}

func TestIsVirtualInterface(t *testing.T) {
	for _, name := range []string{"docker0", "br-3fa1c2d4e5f6", "veth1a2b3c", "virbr0", "cni0", "flannel.1"} {
		if !IsVirtualInterface(name) {
			t.Errorf("%s should be treated as a virtual interface", name)
		}
	}
	for _, name := range []string{"eth0", "enp3s0", "wlan0", "eno1", "tailscale0", "bond0"} {
		if IsVirtualInterface(name) {
			t.Errorf("%s should not be treated as a virtual interface", name)
		}
	}
}
