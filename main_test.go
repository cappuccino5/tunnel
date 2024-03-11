package main

import (
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
	"net/netip"
	"proxy/tunnel/tun"
	"testing"
	"time"
)

func TestTun(t *testing.T) {
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
