package test

import (
	"context"
	"github.com/kelleygo/trojan-go/common"
	"github.com/kelleygo/trojan-go/config"
	"github.com/kelleygo/trojan-go/test/util"
	"github.com/kelleygo/trojan-go/tunnel/freedom"
	"github.com/kelleygo/trojan-go/tunnel/shadowsocks"
	"github.com/kelleygo/trojan-go/tunnel/transport"
	"net"
	"sync"
	"testing"
)

func TestShadowsocks(t *testing.T) {
	port := common.PickPort("tcp", "127.0.0.1")
	transportConfig := &transport.Config{
		LocalHost:  "127.0.0.1",
		LocalPort:  port,
		RemoteHost: "43.200.171.207",
		RemotePort: 28388,
	}
	ctx := config.WithConfig(context.Background(), transport.Name, transportConfig)
	ctx = config.WithConfig(ctx, freedom.Name, &freedom.Config{})
	tcpClient, err := transport.NewClient(ctx, nil)
	common.Must(err)
	cfg := &shadowsocks.Config{
		RemoteHost: "43.200.171.207",
		RemotePort: int(28388),
		Shadowsocks: shadowsocks.ShadowsocksConfig{
			Enabled:  true,
			Method:   "AES-128-GCM",
			Password: "RtajC@14mF&Km",
		},
	}
	ctx = config.WithConfig(ctx, shadowsocks.Name, cfg)

	c, err := shadowsocks.NewClient(ctx, tcpClient)
	common.Must(err)

	wg := sync.WaitGroup{}
	wg.Add(2)
	var conn1 net.Conn
	go func() {
		var err error
		conn1, err = c.DialConn(nil, nil)
		common.Must(err)
		conn1.Write(util.GeneratePayload(1024))
		wg.Done()
	}()
	wg.Wait()
}
