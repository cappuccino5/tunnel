package api

import (
	"proxy/tunnel/auth"
	"proxy/tunnel/config"
	utils "proxy/tunnel/network"
	vpn "proxy/tunnel/proxy"
	"proxy/tunnel/session"
	"strings"
)

// Connect 调用之前必须由前端填充 auth.Prof，建议填充 base.Interface
func Connect() error {
	if strings.Contains(config.Prof.Host, ":") {
		config.Prof.HostWithPort = config.Prof.Host
	} else {
		config.Prof.HostWithPort = config.Prof.Host + ":443"
	}
	// 为适应复杂网络环境，必须能够感知网卡变化，建议由前端获取当前网络信息发送过来，而不是登陆前由 Go 处理
	if !config.Prof.Initialized {
		err := utils.GetLocalInterface()
		if err != nil {
			return err
		}
	}
	
	return SetupTunnel()
}

// SetupTunnel 如果服务端重启断开连接，SessionToken 失效，所以重连即重新建立隧道
func SetupTunnel() error {
	err := auth.InitAuth2()
	// 少写几个 return err
	if err == nil {
		err = auth.PasswordAuth()
		if err != nil {
			return err
		}
		err = vpn.SetupTunnel()
	}
	return err
}

func DisConnect() {
	session.Sess.ActiveClose = true
	if session.Sess.CSess != nil {
		session.Sess.CSess.Close()
	}
}

func Auth() error {
	if strings.Contains(config.Prof.Host, ":") {
		config.Prof.HostWithPort = config.Prof.Host
	} else {
		config.Prof.HostWithPort = config.Prof.Host + ":443"
	}
	err := auth.InitAuth2()
	if err != nil {
		return err
	}
	err = auth.PasswordAuth()
	if err != nil {
		return err
	}
	return nil
}
