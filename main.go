package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"strings"

	dhcp "github.com/krolaw/dhcp4"
)

type Interface struct {
	Name    string
	intNet  *net.Interface
	Giaddr  net.IP
	Dstaddr net.IP
}

func (h *Interface) ServeDHCP(ctx context.Context, p dhcp.Packet, msgType dhcp.MessageType) (answer Answer) {
	answer.MAC = p.CHAddr()
	answer.srvIP = append([]byte(nil), h.Dstaddr...)
	answer.SrcIP = h.Giaddr
	answer.Iface = h.intNet

	switch msgType {

	case dhcp.Discover:
		log.Println("discover ", p.YIAddr(), "from", p.CHAddr())
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
		log.Println("offering from", sip.String(), p.YIAddr(), "to", p.CHAddr())
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
		log.Println("request ", p.YIAddr(), "from", p.CHAddr())
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
		log.Println("ACK from", sip.String(), p.YIAddr(), "to", p.CHAddr())
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
		log.Println("NAK from", p.SIAddr(), p.YIAddr(), "to", p.CHAddr())
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
		iface, err := net.InterfaceByName(interfaceConfig[0])
		if err != nil {
			log.Fatalf("Failed to get interface %s: %v", interfaceConfig[0], err)
		}
		interfaceIP, err := iface.Addrs()
		if err != nil {
			log.Fatalf("Failed to get addresses for interface %s: %v", interfaceConfig[0], err)
		}
		var IPsrc net.IP
		for _, ip := range interfaceIP {
			listenIP, _, err := net.ParseCIDR(ip.String())
			if err != nil {
				continue
			}
			if listenIP.To4() != nil {
				IPsrc = listenIP
			}
		}

		v := Interface{Name: interfaceConfig[0], intNet: iface, Dstaddr: net.ParseIP(interfaceConfig[1]), Giaddr: IPsrc}
		go func() {
			v.run(jobs, ctx)
		}()

		interfaceIP, err = iface.Addrs()
		if err != nil {
			log.Fatalf("Failed to get addresses for interface %s: %v", interfaceConfig[0], err)
		}
		for _, ip := range interfaceIP {
			listenIP, _, err := net.ParseCIDR(ip.String())
			if err != nil {
				continue
			}
			if listenIP.To4() != nil {
				go func(ip net.IP) {
					v.runUnicast(jobs, ip, ctx)
				}(listenIP)
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
