package main

import (
	"context"
	_ "expvar"
	"net"

	dhcp "github.com/krolaw/dhcp4"
)

type job struct {
	p        dhcp.Packet
	msgType  dhcp.MessageType
	handler  Handler
	addr     net.Addr
	dst      net.IP
	localCtx context.Context
}

func doWork(id int, jobe job) {
	var ans Answer

	if ans = jobe.handler.ServeDHCP(jobe.localCtx, jobe.p, jobe.msgType); ans.D != nil {

		switch jobe.msgType {

		case dhcp.Discover:
			sendUnicastDHCP(ans.D, ans.srvIP, ans.SrcIP, 68, 67)
		case dhcp.Offer:
			client, _ := NewRawClient(ans.Iface)
			client.sendDHCP(ans.MAC, ans.D, ans.IP, ans.SrcIP)
			client.Close()
		case dhcp.Request:
			sendUnicastDHCP(ans.D, ans.srvIP, ans.SrcIP, 68, 67)
		case dhcp.ACK:
			client, _ := NewRawClient(ans.Iface)
			client.sendDHCP(ans.MAC, ans.D, ans.IP, ans.SrcIP)
			client.Close()
		}
	}
}

