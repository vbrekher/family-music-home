package external

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
)

func TestPrivateSOCKS5UsesPinnedIP(t *testing.T) {
	origin, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer origin.Close()
	go func() {
		c, e := origin.Accept()
		if e == nil {
			defer c.Close()
			c.Write([]byte("ok"))
		}
	}()
	server, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	dest := make(chan string, 1)
	go func() {
		c, e := server.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		head := make([]byte, 2)
		if _, e = io.ReadFull(c, head); e != nil {
			return
		}
		methods := make([]byte, head[1])
		io.ReadFull(c, methods)
		c.Write([]byte{5, 0})
		req := make([]byte, 4)
		io.ReadFull(c, req)
		if req[3] != 1 {
			return
		}
		ip := make([]byte, 4)
		io.ReadFull(c, ip)
		port := make([]byte, 2)
		io.ReadFull(c, port)
		address := net.JoinHostPort(net.IP(ip).String(), fmt.Sprint(binary.BigEndian.Uint16(port)))
		dest <- address
		up, e := net.Dial("tcp", address)
		if e != nil {
			return
		}
		defer up.Close()
		c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
		io.Copy(c, up)
	}()
	s := testSource(t, "webdav", "https://"+origin.Addr().String())
	s.SocksProxyEnv = "NAVIDROME_SOURCE_PROXY"
	t.Setenv(s.SocksProxyEnv, `{"address":"`+server.Addr().String()+`"}`)
	dial, e := pinnedDial(s)
	if e != nil {
		t.Fatal(e)
	}
	c, e := dial(context.Background(), "tcp", origin.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	out := make([]byte, 2)
	if _, e = io.ReadFull(c, out); e != nil || string(out) != "ok" {
		t.Fatal("SOCKS transport failed", e)
	}
	if <-dest != origin.Addr().String() {
		t.Fatal("proxy target not pinned")
	}
}
