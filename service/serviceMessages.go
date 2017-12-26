package main

import (
	"../message"
	"flag"
	"net"
	"strconv"
)

func main() {
	affectedNodePtr := flag.Int("node", 0, "affected node")
	actionPtr := flag.String("action", "send", "action: send or drop")
	destinationPortPtr := flag.Int("dst", 1, "destination port for send action")
	flag.Parse()
	udp := "udp"

	from, _ := net.ResolveUDPAddr(udp, "127.0.0.1:50000")
	to, _ := net.ResolveUDPAddr(udp, "127.0.0.1:"+strconv.Itoa(40000+*affectedNodePtr))
	connection, _ := net.ListenUDP(udp, from)
	connection.WriteToUDP(message.Message{Type: *actionPtr, Receiver: *destinationPortPtr, Data: "hi there"}.ConvertToJsonMsg(), to)
}
