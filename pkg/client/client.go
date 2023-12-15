package client

import (
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

type Client struct {
	logr.Logger
	conn  net.Conn
	iface *water.Interface
}

func New() *Client {
	z, _ := zap.NewDevelopment()
	l := zapr.NewLogger(z)
	return &Client{Logger: l}
}

func (c *Client) Connect(srv, localAddr string, l byte) {
	var err error
	for {
		c.conn, err = net.Dial("tcp", srv)
		if err != nil {
			c.Error(err, "could not connect to server")
			time.Sleep(time.Second / 10)
			continue
		}
		c.conn.Write(append([]byte("hubproto"), l))
		buf := make([]byte, 4)
		_, err = c.conn.Read(buf)
		if err != nil {
			c.Error(err, "didnt get what i want", "msg", buf)
			continue
		}
		if string(buf[:2]) != "OK" {
			c.Info("didnt get what i want", "msg", buf)
			c.conn.Close()
			continue
		}
		break
	}
	c.startIface(localAddr)
}

func (c *Client) Test(localAddr string) {
	c.startIface(localAddr)
	c.listenIface()
}

func (c *Client) startIface(localAddr string) error {
	var err error
	c.iface, err = water.New(water.Config{})
	if err != nil {
		c.Error(err, "failed to allocate tun interface")
		return err
	}
	c.Info("allocated tun interface")
	link, err := netlink.LinkByName(c.iface.Name())
	if err != nil {
		c.Error(err, "could not get link")
		return err
	}
	addr, _ := netlink.ParseAddr(localAddr)
	netlink.AddrAdd(link, addr)
	netlink.LinkSetUp(link)
	return nil
}

func (c *Client) listenIface() {
	packet := make([]byte, 1536)
	for {
		_, err := c.iface.Read(packet)
		if err != nil {
			break
		}
	}
}

func (c *Client) Handle(r byte) {
	packet := make([]byte, 1536)
	authedPacket := make([]byte, 1545)
	copy(authedPacket, []byte("hubproto"))
	authedPacket[8] = r
	go func() {
		buf := make([]byte, 1536)
		for {
			n, err := c.conn.Read(buf)
			if err != nil {
				c.Error(err, "error reading from tcp conn")
				break
			}
			c.Info("read from server, writing to iface",
				"length", n,
				"header", "\n"+fmt.Sprint(buf[:20])+"\n",
				"bodyStart", "\n"+fmt.Sprint(buf[20:84])+"\n")
			c.iface.Write(buf[:n])
		}
	}()
	for {
		n, err := c.iface.Read(packet)
		if err != nil {
			c.Error(err, "error reading from interface")
			break
		}
		c.Info("read from interface",
			"header", "\n"+fmt.Sprint(packet[:20])+"\n",
			"bodyStart", "\n"+fmt.Sprint(packet[20:84])+"\n")
		copy(authedPacket[9:], packet[:n])
		c.Info("writing to server",
			"authHeader", "\n"+fmt.Sprint(authedPacket[:9])+"\n",
			"header", "\n"+fmt.Sprint(authedPacket[9:29])+"\n",
			"bodyStart", "\n"+fmt.Sprint(authedPacket[29:93])+"\n")
		c.conn.Write(authedPacket[:n+9])
	}
}
