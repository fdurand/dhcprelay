package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/inverse-inc/packetfence/go/log"
	dhcp "github.com/krolaw/dhcp4"

	"flag"
	"fmt"
	"net"
)

var dhcpServers []net.IP
var dhcpGIAddr net.IP

type Interface struct {
	Name    string
	intNet  *net.Interface
	Giaddr  net.IP
	Dstaddr net.IP
}

func (h *Interface) ServeDHCP(ctx context.Context, p dhcp.Packet, msgType dhcp.MessageType) (answer Answer) {
	spew.Dump(h)

	answer.MAC = p.CHAddr()
	answer.srvIP = append([]byte(nil), h.Dstaddr...)
	answer.SrcIP = h.Giaddr
	answer.Iface = h.intNet

	switch msgType {

	case dhcp.Discover:
		fmt.Println("discover ", p.YIAddr(), "from", p.CHAddr())
		// h.m[string(p.XId())] = true
		p2 := dhcp.NewPacket(dhcp.BootRequest)
		p2.SetCHAddr(p.CHAddr())
		p2.SetGIAddr(h.Giaddr)
		p2.SetXId(p.XId())
		p2.SetBroadcast(false)
		for k, v := range p.ParseOptions() {
			p2.AddOption(k, v)
		}
		answer.D = p2
		return answer

	case dhcp.Offer:
		// if !h.m[string(p.XId())] {
		// 	return nil
		// }
		var sip net.IP
		for k, v := range p.ParseOptions() {
			if k == dhcp.OptionServerIdentifier {
				sip = v
			}
		}
		fmt.Println("offering from", sip.String(), p.YIAddr(), "to", p.CHAddr())
		p2 := dhcp.NewPacket(dhcp.BootReply)
		p2.SetXId(p.XId())
		p2.SetFile(p.File())
		p2.SetFlags(p.Flags())
		p2.SetYIAddr(p.YIAddr())
		p2.SetGIAddr(p.GIAddr())
		p2.SetSIAddr(p.SIAddr())
		p2.SetCHAddr(p.CHAddr())
		p2.SetSecs(p.Secs())
		for k, v := range p.ParseOptions() {
			p2.AddOption(k, v)
		}
		answer.IP = p.SIAddr()
		answer.D = p2
		return answer

	case dhcp.Request:
		// h.m[string(p.XId())] = true
		fmt.Println("request ", p.YIAddr(), "from", p.CHAddr())
		p2 := dhcp.NewPacket(dhcp.BootRequest)
		p2.SetCHAddr(p.CHAddr())
		p2.SetFile(p.File())
		p2.SetCIAddr(p.CIAddr())
		p2.SetSIAddr(p.SIAddr())
		p2.SetGIAddr(h.Giaddr)
		p2.SetXId(p.XId())
		p2.SetBroadcast(false)
		for k, v := range p.ParseOptions() {
			p2.AddOption(k, v)
		}
		answer.D = p2
		return answer

	case dhcp.ACK:
		// if !h.m[string(p.XId())] {
		// 	return nil
		// }
		var sip net.IP
		for k, v := range p.ParseOptions() {
			if k == dhcp.OptionServerIdentifier {
				sip = v
			}
		}
		fmt.Println("ACK from", sip.String(), p.YIAddr(), "to", p.CHAddr())
		p2 := dhcp.NewPacket(dhcp.BootReply)
		p2.SetXId(p.XId())
		p2.SetFile(p.File())
		p2.SetFlags(p.Flags())
		p2.SetSIAddr(p.SIAddr())
		p2.SetYIAddr(p.YIAddr())
		p2.SetGIAddr(p.GIAddr())
		p2.SetCHAddr(p.CHAddr())
		p2.SetSecs(p.Secs())
		for k, v := range p.ParseOptions() {
			p2.AddOption(k, v)
		}
		answer.D = p2
		return answer

	case dhcp.NAK:
		// if !h.m[string(p.XId())] {
		// 	return nil
		// }
		fmt.Println("NAK from", p.SIAddr(), p.YIAddr(), "to", p.CHAddr())
		p2 := dhcp.NewPacket(dhcp.BootReply)
		p2.SetXId(p.XId())
		p2.SetFile(p.File())
		p2.SetFlags(p.Flags())
		p2.SetSIAddr(p.SIAddr())
		p2.SetYIAddr(p.YIAddr())
		p2.SetGIAddr(p.GIAddr())
		p2.SetCHAddr(p.CHAddr())
		p2.SetSecs(p.Secs())
		for k, v := range p.ParseOptions() {
			p2.AddOption(k, v)
		}
		answer.D = p2
		return answer

	case dhcp.Release, dhcp.Decline:
		p2 := dhcp.NewPacket(dhcp.BootRequest)
		p2.SetCHAddr(p.CHAddr())
		p2.SetFile(p.File())
		p2.SetCIAddr(p.CIAddr())
		p2.SetSIAddr(p.SIAddr())
		p2.SetGIAddr(h.Giaddr)
		p2.SetXId(p.XId())
		p2.SetBroadcast(false)
		for k, v := range p.ParseOptions() {
			p2.AddOption(k, v)
		}
		answer.D = p2
		return answer
	}
	return answer
}

// func createRelay(in, out string) {
// 	handler := &DHCPHandler{m: make(map[string]bool)}
// 	go ListenAndServeIf(in, out, 67, handler)
// 	ListenAndServeIf(out, in, 68, handler)
// }

var ctx = context.Background()

func main() {
	flagConfig := flag.String("config", "interface:giaddr,interface2:giaddr", "Couple of interface and giaddr, like eth1:192.168.0.1,eth2:192.168.2.1")
	flag.Parse()

	ctx = log.LoggerNewContext(ctx)

	// Queue value
	var (
		maxQueueSize = 100
		maxWorkers   = 100
	)

	// create job channel
	jobs := make(chan job, maxQueueSize)

	// create workers
	for i := 1; i <= maxWorkers; i++ {
		go func(i int) {
			for j := range jobs {
				doWork(i, j)
			}
		}(i)
	}

	result := strings.Split(*flagConfig, ",")
	for i := range result {
		interfaceConfig := strings.Split(result[i], ":")
		iface, _ := net.InterfaceByName(interfaceConfig[0])
		interfaceIP, _ := iface.Addrs()
		var IPsrc net.IP
		for _, ip := range interfaceIP {
			ip := ip
			listenIP, _, _ := net.ParseCIDR(ip.String())
			if listenIP.To4() != nil {
				IPsrc = listenIP
			}
		}

		v := Interface{Name: interfaceConfig[0], intNet: iface, Dstaddr: net.ParseIP(interfaceConfig[1]), Giaddr: IPsrc}
		spew.Dump(v)
		go func() {
			v.run(jobs, ctx)
		}()

		interfaceIP, _ = iface.Addrs()
		for _, ip := range interfaceIP {
			ip := ip
			listenIP, _, _ := net.ParseCIDR(ip.String())
			if listenIP.To4() != nil {
				go func() {
					spew.Dump(listenIP)
					v.runUnicast(jobs, listenIP, ctx)
				}()
			}
		}
	}

	http.ListenAndServe("localhost:6061", nil)

}

// Broadcast Listener
func (v *Interface) run(jobs chan job, ctx context.Context) {

	// handler := &DHCPHandler{m: make(map[string]bool)}
	// go ListenAndServeIf(in, out, 67, handler)
	// ListenAndServeIf(out, in, 68, handler)

	ListenAndServeIf(v.Name, v, jobs, ctx)
}

// Unicast listener
func (v *Interface) runUnicast(jobs chan job, ip net.IP, ctx context.Context) {

	ListenAndServeIfUnicast(v.Name, v, jobs, ip, ctx)
}
