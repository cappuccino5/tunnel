package main

import (
	"dev.risinghf.com/go/framework/log"
	"encoding/json"
	"flag"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
	"net/netip"
	"os"
	"os/signal"
	"proxy/tunnel/config"
	"proxy/tunnel/network"
	_ "proxy/tunnel/network/waterutil"
	"proxy/tunnel/proxy_client"
	"proxy/tunnel/session"
	_ "proxy/tunnel/static"
	"proxy/tunnel/tun"
	"syscall"
	"time"
)

func main2() {
	ifname := "MyNIC"
	dev, err := tun.CreateTUN(ifname, 0)
	if err != nil {
		panic(err)
	}
	defer dev.Close()
	// 保存原始设备句柄
	nativeTunDevice := dev.(*tun.NativeTun)
	
	// 获取LUID用于配置网络
	link := winipcfg.LUID(nativeTunDevice.LUID())
	
	ip, err := netip.ParsePrefix("10.0.0.77/24")
	if err != nil {
		panic(err)
	}
	err = link.SetIPAddresses([]netip.Prefix{ip})
	if err != nil {
		panic(err)
	}
	// 配置虚拟网段路由
	// err = link.SetRoutes([]*winipcfg.RouteData{
	//	{net.IPNet{IP: ip.Mask(cidrMask), Mask: cidrMask}, m.gateway, 0},
	//})
	time.Sleep(time.Second * 30)
}

func main() {
	log.SetLogLevel("debug")
	flagSet := flag.NewFlagSet("project-start", flag.ExitOnError)
	testRun := flagSet.String("test", "", "测试代理链路是否正常")
	// 打印
	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("%s", *testRun)
	err = config.InitAuth2()
	if err != nil {
		log.Error(err)
		return
	}
	err = Connect()
	if err != nil {
		log.Error(err)
		return
	}
	
	time.Sleep(time.Second * 30)
	
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(quit)
	
	for {
		select {
		case <-quit:
			log.Info("exit main ")
			return
		}
	}
}

func SetupTunnel() error {
	session := session.Sess.NewConnSession()
	// TODO 本地网卡的地址和DNS 子网掩码配置
	//session.VPNAddress = config.Prof.ServiceAddr()
	session.VPNAddress = "192.168.159.12"
	session.ServerAddress = config.Prof.ServiceAddr()
	session.LocalAddress = config.LocalInterface.Ip4
	session.DNS = []string{"8.8.8.8", "114.114.114.114"}
	session.VPNMask = "255.255.255.0"
	session.TunName = "rxhf egde"
	
	sessb, _ := json.Marshal(session)
	log.Info("sess:", string(sessb))
	err := proxy_client.SetupTun(session)
	if err != nil {
		return err
	}
	
	go proxy_client.TlsVmessChannel(config.Conn, config.BufR, session)
	session.ReadDeadTimer()
	err = network.SetRoutes(session.ServerAddress, &[]string{}, &[]string{})
	if err != nil {
		config.Conn.Close()
		session.Close()
		return err
	}
	
	return nil
}

// Connect 调用之前必须由前端填充 auth.Prof，建议填充 base.Interface
func Connect() error {
	// 为适应复杂网络环境，必须能够感知网卡变化，建议由前端获取当前网络信息发送过来，而不是登陆前由 Go 处理
	if !config.Prof.Initialized {
		err := network.GetLocalInterface()
		if err != nil {
			return err
		}
	}
	return SetupTunnel()
}
