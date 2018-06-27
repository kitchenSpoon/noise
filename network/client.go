package network

import (
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/jpillora/backoff"
	"github.com/perlin-network/noise/network/rpc"
	"github.com/perlin-network/noise/peer"
	"github.com/perlin-network/noise/protobuf"
	"github.com/pkg/errors"
	"github.com/xtaci/kcp-go"
	"github.com/xtaci/smux"
	"reflect"
	"time"
)

// Represents a single incomingStream peer client.
type PeerClient struct {
	Network *Network

	Id *peer.ID

	Session *smux.Session

	Backoff *backoff.Backoff
}

func createPeerClient(network *Network) *PeerClient {
	return &PeerClient{Network: network, Backoff: &backoff.Backoff{}}
}

func (c *PeerClient) establishConnection(address string) error {
	if c.Session != nil {
		return errors.New("connection already established")
	}

	dialer, err := kcp.DialWithOptions(address, nil, 10, 3)

	// Failed to connect. Continue.
	if err != nil {
		glog.Error(err)
		return err
	}

	c.Session, err = smux.Client(dialer, muxConfig())

	// Failed to open session. Continue.
	if err != nil {
		glog.Error(err)
		return err
	}

	return nil
}

func (c *PeerClient) reestablishConnection() error {
	if c.Session != nil && !c.Session.IsClosed() {
		err := c.Session.Close()
		if err != nil {
			glog.Error(err)
			return err
		}
	}

	c.Backoff.Reset()
	c.Session = nil
	maxAttempts := 5
	attempt := 0

	for {
		if attempt >= maxAttempts {
			c.close()
			return errors.New("unable to reestablish connection")
		}
		attempt++

		err := c.establishConnection(c.Id.Address)
		if err != nil {
			d := c.Backoff.Duration()
			time.Sleep(d)
			continue
		}

		break
	}

	return nil
}

func (c *PeerClient) close() {
	// Disconnect the user.
	if c.Id != nil {
		if c.Network.Routes.PeerExists(*c.Id) {
			c.Network.Routes.RemovePeer(*c.Id)
			glog.Infof("Peer %s has disconnected.", c.Id.Address)
		}
	}
}

func (c *PeerClient) handleMessage(stream *smux.Stream) {
	// Clean up resources.
	defer stream.Close()

	msg, err := c.receiveMessage(stream)

	// Failed to receive message.
	if err != nil {
		glog.Error(err)
		return
	}

	// Derive, set the peer ID, connect to the peer, and additionally
	// store the peer.
	id := peer.ID(*msg.Sender)

	if c.Id == nil {
		c.Id = &id

		err := c.establishConnection(id.Address)

		// Could not connect to peer; disconnect.
		if err != nil {
			glog.Errorf("Failed to connect to peer %s err=[%+v]\n", id.Address, err)
			return
		}
	} else if !c.Id.Equals(id) {
		// Peer sent message with a completely different ID (???)
		glog.Errorf("Message signed by peer %s but client is %s", c.Id.Address, id.Address)
		return
	}

	// Update routing table w/ peer's ID.
	c.Network.Routes.Update(id)

	// Unmarshal protobuf.
	var ptr ptypes.DynamicAny
	if err := ptypes.UnmarshalAny(msg.Message, &ptr); err != nil {
		glog.Error(err)
		return
	}

	// Check if the received request has a message processor. If exists, execute it.
	name := reflect.TypeOf(ptr.Message).String()
	processor, exists := c.Network.Processors.Load(name)

	glog.Infof("%s sent response of type %s", c.Id.Address, name)

	if exists {
		processor := processor.(MessageProcessor)

		// Create message execution context.
		ctx := new(MessageContext)
		ctx.client = c
		ctx.stream = stream
		ctx.message = ptr.Message

		// Process request.
		err := processor.Handle(ctx)
		if err != nil {
			glog.Errorf("An error occurred handling %x: %x", name, err)
		}
	} else {
		glog.Warning("Unknown message type received:", name)
	}
}

// Marshals message into proto.Message and signs it with this node's private key.
// Errors if the message is null.
func (c *PeerClient) prepareMessage(message proto.Message) (*protobuf.Message, error) {
	if message == nil {
		return nil, errors.New("message is null")
	}

	raw, err := ptypes.MarshalAny(message)
	if err != nil {
		return nil, err
	}

	id := protobuf.ID(c.Network.ID)

	signature, err := c.Network.Keys.Sign(raw.Value)
	if err != nil {
		return nil, err
	}

	msg := &protobuf.Message{
		Message:   raw,
		Sender:    &id,
		Signature: signature,
	}

	return msg, nil
}

// Asynchronously emit a message to a given peer.
func (c *PeerClient) Tell(message proto.Message) error {
	if c.Session == nil {
		return errors.New("client session nil")
	}

	// Open a new stream.
	stream, err := c.Session.OpenStream()
	if err != nil {
		return err
	}
	defer stream.Close()

	// Send message bytes.
	err = c.sendMessage(stream, message)
	if err != nil {
		glog.Error(err)
		return err
	}

	return nil
}

// Request requests for a response for a request sent to a given peer.
func (c *PeerClient) Request(req *rpc.Request) (proto.Message, error) {
	if c.Session == nil {
		return nil, errors.New("client session nil")
	}

	// Open a new stream.
	stream, err := c.Session.OpenStream()
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	stream.SetDeadline(time.Now().Add(req.Timeout))

	// Send request bytes.
	err = c.sendMessage(stream, req.Message)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	// Await for response bytes.
	res, err := c.receiveMessage(stream)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	// Unmarshal response protobuf.
	var ptr ptypes.DynamicAny
	if err := ptypes.UnmarshalAny(res.Message, &ptr); err != nil {
		return nil, err
	}

	return ptr.Message, nil
}
