package config

import (
	"bufio"
	"context"
	"dev.risinghf.com/go/framework/log"
	"github.com/kelleygo/trojan-go/config"
	"github.com/kelleygo/trojan-go/tunnel/freedom"
	"github.com/kelleygo/trojan-go/tunnel/shadowsocks"
	xtls "github.com/kelleygo/trojan-go/tunnel/tls"
	"github.com/kelleygo/trojan-go/tunnel/transport"
	"net"
)

var (
	LocalInterface = &Interface{}
	Prof           = &Profile{Initialized: false}
	Conn           net.Conn
	// Conn2        net.Conn
	// Conn3        net.Conn
	BufR *bufio.Reader
	// BufR2        *bufio.Reader
	// BufR3        *bufio.Reader
	reqHeaders   = make(map[string]string)
	WebVpnCookie string
)

// Profile 模板变量字段必须导出，虽然全局，但每次连接都被重置
type Profile struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`

	Initialized bool
}

// Interface 应该由外部接口设置
type Interface struct {
	Name    string `json:"name"`
	Ip4     string `json:"ip4"`
	Mac     string `json:"mac"`
	Gateway string `json:"gateway"`
}

func (this *Profile) ServiceAddr() string {
	ips, err := net.LookupIP(this.Host)
	if err != nil {
		log.Error(err)
		return ""
	}
	var addr string
	for _, ip := range ips {
		addr = ip.String()
		break
	}
	return addr
}

func InitAuth2() error {
	host := "43.200.171.207"
	port := 28388
	Prof.Host = host
	Prof.Port = port

	tlsConfig := &xtls.Config{
		TLS: xtls.TLSConfig{
			Verify:      false,
			SNI:         host,
			Fingerprint: "",
		},
	}
	transportConfig := &transport.Config{
		RemoteHost: host,
		RemotePort: port,
	}
	shadowsocksConfig := &shadowsocks.Config{
		RemoteHost: host,
		RemotePort: port,
		Shadowsocks: shadowsocks.ShadowsocksConfig{
			Enabled:  true,
			Method:   "AES-128-GCM",
			Password: "RtajC@14mF&Km",
		},
	}

	ctx := config.WithConfig(context.Background(), transport.Name, transportConfig)
	ctx = config.WithConfig(ctx, xtls.Name, tlsConfig)
	ctx = config.WithConfig(ctx, shadowsocks.Name, shadowsocksConfig)
	ctx = config.WithConfig(ctx, freedom.Name, &freedom.Config{})
	tcpClient, err := transport.NewClient(ctx, nil)
	if err != nil {
		return err
	}
	//tlsClient, err := xtls.NewClient(ctx, tcpClient)
	//if err != nil {
	//	return err
	//}
	ssClient, err := shadowsocks.NewClient(ctx, tcpClient)
	if err != nil {
		return err
	}
	Conn, err = ssClient.DialConn(nil, nil)
	if err != nil {
		return err
	}
	BufR = bufio.NewReader(Conn)
	return nil
}
