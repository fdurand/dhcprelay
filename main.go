package main

import (
	"context"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/inverse-inc/packetfence/go/log"
	dhcp "github.com/krolaw/dhcp4"

	"flag"
	"fmt"
	"net"
	"strings"
)

var dhcpServers []net.IP
var dhcpGIAddr net.IP

type DHCPHandler struct {
	m map[string]bool
}

func (h *Interface) ServeDHCP(ctx context.Context, p dhcp.Packet, msgType dhcp.MessageType) (answer Answer) {
	answer.MAC = p.CHAddr()
	answer.SrcIP = h.Giaddr
	answer.Iface = h.intNet
	switch msgType {

	case dhcp.Discover:
		fmt.Println("discover ", p.YIAddr(), "from", p.CHAddr())
		// h.m[string(p.XId())] = true
		p2 := dhcp.NewPacket(dhcp.BootRequest)
		p2.SetCHAddr(p.CHAddr())
		p2.SetGIAddr(dhcpGIAddr)
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
		p2.SetGIAddr(dhcpGIAddr)
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
		p2.SetGIAddr(dhcpGIAddr)
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

type Interface struct {
	Name   string
	intNet *net.Interface
	Giaddr net.IP
}

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
	// config := strings.Fields(*flagConfig)
	result := strings.Split(*flagConfig, ",")
	for i := range result {
		spew.Dump(result)
		interfaceConfig := strings.Split(result[i], ":")
		iface, _ := net.InterfaceByName(interfaceConfig[0])
		v := Interface{Name: interfaceConfig[0], intNet: iface, Giaddr: net.ParseIP(interfaceConfig[1])}
		spew.Dump(v)
		go func() {
			v.run(jobs, ctx)
		}()

		interfaceIP, _ := iface.Addrs()
		for _, ip := range interfaceIP {
			go func() {
				v.runUnicast(jobs, net.ParseIP(ip.String()), ctx)
			}()
		}
		http.ListenAndServe("localhost:6060", nil)
	}

	// servers := strings.Fields(*flagServers)
	// for _, s := range servers {
	// 	dhcpServers = append(dhcpServers, net.ParseIP(s))
	// }
	// dhcpGIAddr = net.ParseIP(*flagBindIP)
	// if dhcpGIAddr == nil {
	// 	panic("giaddr needed")
	// }
	// createRelay(*flagInInt, *flagOutInt)
}

// Broadcast Listener
func (h *Interface) run(jobs chan job, ctx context.Context) {

	// handler := &DHCPHandler{m: make(map[string]bool)}
	// go ListenAndServeIf(in, out, 67, handler)
	// ListenAndServeIf(out, in, 68, handler)

	ListenAndServeIf(h.Name, h, jobs, ctx)
}

// Unicast listener
func (h *Interface) runUnicast(jobs chan job, ip net.IP, ctx context.Context) {

	ListenAndServeIfUnicast(h.Name, h, jobs, ip, ctx)
}
