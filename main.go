package main

import (
	"dev.risinghf.com/go/framework/log"
	"net/http"
	"os"
	"os/signal"
	"proxy/tunnel/api"
	_ "proxy/tunnel/auth"
	"proxy/tunnel/config"
	_ "proxy/tunnel/network/waterutil"
	_ "proxy/tunnel/static"
	"runtime"
	"syscall"
)

func main() {
	log.SetLogLevel("debug")
	
	config.Prof = &config.Profile{
		Host:        "54.198.47.122:443",
		Username:    "rhf2s027l",
		Password:    "risinghf",
		Group:       "ops",
		Scheme:      "https://",
		Initialized: false,
		AppVersion:  config.Cfg.AgentVersion,
		HeaderParam: make(http.Header),
	}
	// 三次认证请求必须要加的请求头
	config.Prof.HeaderParam["X-Transcend-Version"] = []string{"1"}
	config.Prof.HeaderParam["X-Aggregate-Auth"] = []string{"1"}
	if config.Cfg.CiscoCompat {
		config.Prof.HeaderParam["User-Agent"] = []string{"AnyConnect" + runtime.GOOS + " " + config.Cfg.AgentVersion}
	} else {
		config.Prof.HeaderParam["User-Agent"] = []string{config.Cfg.AgentName + " " + runtime.GOOS + " " + config.Cfg.AgentVersion}
	}
	
	log.WithFields(config.Prof).Info("config init")
	
	err := api.Connect()
	if err != nil {
		log.Error(err)
		return
	}
	defer api.DisConnect()
	
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
	
	//flagSet := flag.NewFlagSet("project-start", flag.ExitOnError)
	//testRun := flagSet.String("test", "", "测试代理链路是否正常")
	//// 打印
	//err := flagSet.Parse(os.Args[1:])
	//if err != nil {
	//	log.Fatal(err)
	//}
}
