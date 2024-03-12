package test

import (
	"bufio"
	"context"
	"dev.risinghf.com/go/framework/log"
	"github.com/kelleygo/trojan-go/common"
	"github.com/kelleygo/trojan-go/config"
	"github.com/kelleygo/trojan-go/tunnel/freedom"
	"github.com/kelleygo/trojan-go/tunnel/shadowsocks"
	"github.com/kelleygo/trojan-go/tunnel/transport"
	"net"
	"sync"
	"testing"
	"time"
)

func TestShadowsocks(t *testing.T) {
	transportConfig := &transport.Config{
		RemoteHost: "v.hujian.xyz",
		RemotePort: 2443,
		TransportPlugin: transport.TransportPluginConfig{
			Enabled: true,
			Type:    "shadowsocks",
			Command: "./v2ray-plugin",
			Option:  "",
			Arg:     []string{"-host", "v.hujian.xyz"},
			Env:     nil,
		},
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
	var bufR *bufio.Reader
	go func() {
		var err error
		conn1, err = c.DialConn(nil, nil)
		common.Must(err)
		bufR = bufio.NewReader(conn1)
		time.Sleep(time.Second * 5)
		_, err = conn1.Write([]byte{0x05, 0x00, 0x00})
		common.Must(err)
		tempW := []byte{0x05, 0x45, 0x00, 0x00, 0x38, 0x0b, 0x28, 0x00, 0x00, 0x80, 0x11, 0xea, 0xf3, 0xc0, 0xa8, 0x9f, 0x0c, 0x72, 0x72, 0x72, 0x72, 0xf8, 0xf7, 0x00, 0x35, 0x00, 0x24, 0x73,
			0x81, 0x99, 0x40, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x64, 0x6e, 0x73, 0x06, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x00, 0x00, 0x01, 0x00, 0x01}
		conn1.Write(tempW)
		wg.Done()
	}()
	
	go func() {
		defer wg.Done()
		var data = make([]byte, 104)
		for {
			if bufR == nil {
				continue
			}
			bytesReceived, err := bufR.Read(data) // 服务器没有数据返回时，会阻塞
			if err != nil {
				//if err == io.EOF {
				//	continue
				//}
				log.Error("bytesReceived error:", err)
				return
			}
			log.Info("bytesReceived:", bytesReceived)
		}
	}()
	wg.Wait()
}
