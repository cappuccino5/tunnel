package main

import (
	"dev.risinghf.com/go/framework/log"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"proxy/tunnel/api"
	"proxy/tunnel/config"
	_ "proxy/tunnel/network/waterutil"
	_ "proxy/tunnel/static"
	"runtime"
	"syscall"
)

func main() {
	log.SetLogLevel("debug")
	config.Prof = &config.Profile{
		Host:        "54.221.65.219:8800",
		Username:    "rhf2s027l",
		Password:    "risinghf",
		Group:       "ops",
		Initialized: false,
		HeaderParam: make(http.Header),
	}
	config.Prof.HeaderParam["User-Agent"] = []string{"edge " + runtime.GOOS}
	conf, _ := json.Marshal(config.Prof)
	log.Info("config init:", string(conf))
	err := api.Connect()
	if err != nil {
		log.Error(err)
		return
	}
	
	//flagSet := flag.NewFlagSet("project-start", flag.ExitOnError)
	//testRun := flagSet.String("test", "", "测试代理链路是否正常")
	//// 打印
	//err := flagSet.Parse(os.Args[1:])
	//if err != nil {
	//	log.Fatal(err)
	//}
	//log.Infof("%s", *testRun)
	//err = auth.InitAuth2()
	//if err != nil {
	//	log.Error(err)
	//	return
	//}
	//err = Connect()
	//if err != nil {
	//	log.Error(err)
	//	return
	//}
	//
	//time.Sleep(time.Second * 30)
	
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

//
//func SetupTunnel() error {
//	session := session.Sess.NewConnSession()
//	// TODO 本地网卡的地址和DNS 子网掩码配置
//	//session.VPNAddress = config.Prof.ServiceAddr()
//	session.VPNAddress = "192.168.159.12"
//	session.ServerAddress = config.Prof.ServiceAddr()
//	session.LocalAddress = config.LocalInterface.Ip4
//	session.DNS = []string{"8.8.8.8", "114.114.114.114"}
//	session.VPNMask = "255.255.255.0"
//	session.TunName = "rxhf egde"
//
//	sessb, _ := json.Marshal(session)
//	log.Info("sess:", string(sessb))
//	err := proxy_client.SetupTun(session)
//	if err != nil {
//		return err
//	}
//
//	go proxy_client.TlsVmessChannel(config.Conn, config.BufR, session)
//	session.ReadDeadTimer()
//	err = network.SetRoutes(session.ServerAddress, &[]string{}, &[]string{})
//	if err != nil {
//		config.Conn.Close()
//		session.Close()
//		return err
//	}
//
//	return nil
//}
//
//// Connect 调用之前必须由前端填充 auth.Prof，建议填充 base.Interface
//func Connect() error {
//	// 为适应复杂网络环境，必须能够感知网卡变化，建议由前端获取当前网络信息发送过来，而不是登陆前由 Go 处理
//	if !config.Prof.Initialized {
//		err := network.GetLocalInterface()
//		if err != nil {
//			return err
//		}
//	}
//	return SetupTunnel()
//}
