package main

import (
	"flag"
	"strings"

	"github.com/golang/glog"
	"github.com/perlin-network/noise/crypto/signing/ed25519"
	"github.com/perlin-network/noise/network"
	"github.com/perlin-network/noise/network/backoff"
	"github.com/perlin-network/noise/network/builders"
	"github.com/perlin-network/noise/network/discovery"
	"github.com/perlin-network/noise/network/nat"
)

func main() {
	// glog defaults to logging to a file, override this flag to log to console for testing
	flag.Set("logtostderr", "true")

	// process other flags
	portFlag := flag.Int("port", 3000, "port to listen to")
	hostFlag := flag.String("host", "localhost", "host to listen to")
	protocolFlag := flag.String("protocol", "tcp", "protocol to use (kcp/tcp)")
	peersFlag := flag.String("peers", "", "peers to connect to")
	upnpFlag := flag.Bool("upnp", false, "enable upnp")
	reconnectFlag := flag.Bool("reconnect", false, "enable reconnections")
	flag.Parse()

	port := uint16(*portFlag)
	host := *hostFlag
	protocol := *protocolFlag
	upnpEnabled := *upnpFlag
	reconnectEnabled := *reconnectFlag
	peers := strings.Split(*peersFlag, ",")

	keys := ed25519.RandomKeyPair()

	glog.Infof("Private Key: %s", keys.PrivateKeyHex())
	glog.Infof("Public Key: %s", keys.PublicKeyHex())

	builder := builders.NewNetworkBuilder()
	builder.SetKeys(keys)
	builder.SetAddress(network.FormatAddress(protocol, host, port))

	// Register UPnP plugin.
	if upnpEnabled {
		nat.RegisterPlugin(builder)
	}

	// Register the reconnection plugin
	if reconnectEnabled {
		builder.AddPlugin(new(backoff.Plugin))
	}

	// Register peer discovery plugin.
	builder.AddPlugin(new(discovery.Plugin))

	net, err := builder.Build()
	if err != nil {
		glog.Fatal(err)
		return
	}

	go net.Listen()

	if len(peers) > 0 {
		net.Bootstrap(peers...)
	}

	select {}

	glog.Flush()
}
