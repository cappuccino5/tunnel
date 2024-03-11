package main

import (
	"bufio"
	"dev.risinghf.com/go/framework/log"
	"encoding/hex"
	"flag"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
	"net/netip"
	"os"
	"os/signal"
	"proxy/tunnel/network"
	_ "proxy/tunnel/network/waterutil"
	_ "proxy/tunnel/static"
	"proxy/tunnel/tun"
	"runtime"
	"strings"
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
	flagSet := flag.NewFlagSet("project-start", flag.ExitOnError)
	testRun := flagSet.String("test", "", "测试代理链路是否正常")
	// 打印
	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("%s", *testRun)
	err = SetupTunnel()
	if err != nil {
		log.Error(err)
	}
	time.Sleep(time.Second * 30)

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(quit)

	for {
		select {
		case <-quit:
			log.Info("exit main ")
		}
	}
}

func SetupTunnel() error {

	// 1 发送 vmess 请求
	//err := req.Write(auth.Conn)
	//if err != nil {
	//	auth.Conn.Close()
	//	return err
	//}
	//var resp *http.Response
	//// resp.Body closed when tlsChannel exit
	//resp, err = http.ReadResponse(auth.BufR, req)
	//if err != nil {
	//	auth.Conn.Close()
	//	return err
	//}
	//
	//if resp.StatusCode != http.StatusOK {
	//	auth.Conn.Close()
	//	return fmt.Errorf("tunnel negotiation failed %s", resp.Status)
	//}

	//err = setupTun(cSess)
	//if err != nil {
	//	auth.Conn.Close()
	//	cSess.Close()
	//	return err
	//}
	//log.Info("tls channel negotiation succeeded")
	//// 只有网卡设置成功才会进行下一步
	//// https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-03#section-2.1.4
	//go tlsChannel(auth.Conn, auth.BufR, cSess, resp)
	//if cSess.DTLSPort != "" {
	//	// https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-03#section-2.1.5
	//	go dtlsChannel(cSess)
	//}
	//cSess.DPDTimer()
	//cSess.ReadDeadTimer()
	//
	//为了靠谱，不再异步设置，路由多的话可能要等等
	//err = utils.SetRoutes(cSess.ServerAddress, &cSess.SplitInclude, &cSess.SplitExclude)
	//if err != nil {
	//	auth.Conn.Close()
	//	cSess.Close()
	//}

	session := Sess.NewConnSession()
	session.VPNAddress = "54.221.65.219:1443"
	err := setupTun(session)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(strings.NewReader("hello \n world"))

	tlsVmessChannel(nil, reader, session)
	return nil
}

func setupTun(cSess *ConnSession) error {
	if runtime.GOOS == "windows" {
		cSess.TunName = "Egde VPN"
	} else {
		cSess.TunName = "egde VPN"
	}
	if cSess.MTU == 0 {
		cSess.MTU = 1399
	}
	dev, err := tun.CreateTUN(cSess.TunName, cSess.MTU)
	if err != nil {
		log.Error("failed to creates a new tun interface")
		return err
	}
	log.Info("tun device:", cSess.TunName)
	tun.NativeTunDevice = dev.(*tun.NativeTun)

	// 不可并行
	err = network.ConfigInterface(cSess.TunName, cSess.VPNAddress, cSess.VPNMask, cSess.DNS)
	if err != nil {
		_ = dev.Close()
		return err
	}

	go tunToPayloadOut(dev, cSess) // read from apps
	go payloadInToTun(dev, cSess)  // write to apps
	return nil
}

// Step 3
// 网络栈将应用数据包转给 tun 后，该函数从 tun 读取数据包，放入 cSess.PayloadOutTLS 或 cSess.PayloadOutDTLS
// 之后由 payloadOutTLSToServer 或 payloadOutDTLSToServer 调整格式，发送给服务端
func tunToPayloadOut(dev tun.Device, cSess *ConnSession) {
	// tun 设备读错误
	defer func() {
		log.Info("tun to payloadOut exit")
		_ = dev.Close()
	}()
	var (
		err error
		n   int
	)

	for {
		// 从池子申请一块内存，存放到 PayloadOutTLS 或 PayloadOutDTLS，在 payloadOutTLSToServer 或 payloadOutDTLSToServer 中释放
		// 由 payloadOutTLSToServer 或 payloadOutDTLSToServer 添加 header 后发送出去
		pl := getPayloadBuffer()
		n, err = dev.Read(pl.Data, 0) // 如果 tun 没有 up，会在这等待
		if err != nil {
			log.Error("tun to payloadOut error:", err)
			return
		}

		// 更新数据长度
		pl.Data = (pl.Data)[:n]

		log.Debug("tunToPayloadOut", hex.EncodeToString(pl.Data))
		select {
		case cSess.PayloadOut <- pl:
		case <-cSess.CloseChan:
			return
		}
	}
}

// Step 22
// 读取 tlsChannel、dtlsChannel 放入 cSess.PayloadIn 的数据包（由服务端返回，已调整格式），写入 tun，网络栈交给应用
func payloadInToTun(dev tun.Device, cSess *ConnSession) {
	// tun 设备写错误或者cSess.CloseChan
	defer func() {
		log.Info("payloadIn to tun exit")
		// 可能由写错误触发，和 tunRead 一起，只要有一处确保退出 cSess 即可
		// 如果由外部触发，cSess.Close() 因为使用 sync.Once，所以没影响
		cSess.Close()
		_ = dev.Close()
	}()

	var (
		err error
		pl  *Payload
	)

	for {
		select {
		case pl = <-cSess.PayloadIn:
		case <-cSess.CloseChan:
			return
		}

		_, err = dev.Write(pl.Data, 0)
		if err != nil {
			log.Error("payloadIn to tun error:", err)
			return
		}

		// log.Debug("payloadInToTun")

		// 释放由 serverToPayloadIn 申请的内存
		putPayloadBuffer(pl)
	}
}
