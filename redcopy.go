package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/smallnest/ringbuffer"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

var (
	pcapFile      = flag.String("f", "", "offline pcap file. If is empty redcopy use live capture")
	device        = flag.String("i", "eth0", "captured device")
	captureAddr   = flag.String("addr", "", "captured address and port, for example, 127.0.0.1:8080")
	copyserver    = flag.String("s", "", "redis server for receiving captured redis commands")
	debugResponse = flag.Bool("d", false, "should display responses from copyserver")
)

var (
	handle *pcap.Handle
	err    error
	conns  map[string]*connection
	mu     sync.RWMutex
)

// redcopy是一个redis流量复制工具，它将online的redis服务器上的请求流量复制到redcopy server,
// redcopy server可以是一个真正的redis服务器，也可以是一个中转服务器，由中转服务器进行分发。
func main() {
	flag.Parse()

	conns = make(map[string]*connection)

	var w io.Writer
	if *copyserver != "" {
		conn, err := net.DialTimeout("tcp", *copyserver, 10*time.Second)
		if err != nil {
			log.Fatalf("failed to dial *v: %v", *copyserver, err)
		}
		w = conn
		defer conn.Close()

		go discardConnRead(conn)
	} else {
		w = os.Stdout
	}

	if *pcapFile != "" {
		handle, err = pcap.OpenOffline(*pcapFile)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		handle, err = pcap.OpenLive(*device, 1522, false, 30*time.Second)
		if err != nil {
			log.Fatal(err)
		}
	}
	defer handle.Close()

	if *captureAddr != "" {
		host, port, err := net.SplitHostPort(*captureAddr)
		if err != nil {
			log.Fatalf("invalid captured address: %v", err)
		}

		var filter = "tcp and dst port " + port + " and dst host " + host
		err = handle.SetBPFFilter(filter)
		if err != nil {
			log.Fatalf("failed to set filter: %v", err)
		}
	}
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		var clientIP, clientPort string
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer != nil {
			ip, _ := ipLayer.(*layers.IPv4)
			clientIP = ip.SrcIP.String()
		}
		if clientIP == "" {
			ipLayer = packet.Layer(layers.LayerTypeIPv6)
			if ipLayer != nil {
				ip, _ := ipLayer.(*layers.IPv6)
				clientIP = ip.SrcIP.String()
			}
		}

		tcpLayer := packet.Layer(layers.LayerTypeTCP)
		if tcpLayer != nil {
			tcp, _ := tcpLayer.(*layers.TCP)
			clientPort = strconv.Itoa(int(tcp.SrcPort))
		}

		key := clientIP + ":" + clientPort
		mu.RLock()
		c := conns[key]
		mu.RUnlock()
		if c == nil {
			c = &connection{
				buf: ringbuffer.New(1024 * 1024),
				closeCallback: func(err error) {
					mu.Lock()
					delete(conns, key)
					mu.Unlock()
				},
				parseCallBack: func(raw []byte) {
					w.Write(raw)
				},
			}
			mu.Lock()
			conns[key] = c
			mu.Unlock()
			go c.Start()
		}

		applicationLayer := packet.ApplicationLayer()
		if applicationLayer != nil {
			c.buf.Write(applicationLayer.Payload())
		}

		if err := packet.ErrorLayer(); err != nil {
			fmt.Printf("Error decoding some part of the packet: %v", err)
		}
	}

	select {}
}

func discardConnRead(conn net.Conn) {
	w := ioutil.Discard
	if *debugResponse {
		w = os.Stdout
	}
	for {
		io.Copy(w, conn)
	}
}
