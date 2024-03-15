package auth

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/kelleygo/trojan-go/config"
	"github.com/kelleygo/trojan-go/log"
	"github.com/kelleygo/trojan-go/tunnel/freedom"
	"github.com/kelleygo/trojan-go/tunnel/shadowsocks"
	xtls "github.com/kelleygo/trojan-go/tunnel/tls"
	"github.com/kelleygo/trojan-go/tunnel/transport"
	"io"
	"net/http"
	conf "proxy/tunnel/config"
	"proxy/tunnel/models"
	"proxy/tunnel/network"
	"proxy/tunnel/session"
	"strconv"
	"strings"
	"text/template"
)

var (
	WebVpnCookie string
)

const (
	tplInit = iota
	tplAuthReply
)

// InitAuth 确定用户组和服务端认证地址 AuthPath
func InitAuth2() error {
	addr := conf.Prof.HostWithPort
	addrArr := strings.Split(addr, ":")
	host := addrArr[0]
	port, _ := strconv.Atoi(addrArr[1])
	
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
			Password: "password",
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
	tlsClient, err := xtls.NewClient(ctx, tcpClient)
	if err != nil {
		return err
	}
	ssClient, err := shadowsocks.NewClient(ctx, tlsClient)
	if err != nil {
		return err
	}
	conf.Conn, err = ssClient.DialConn(nil, nil)
	if err != nil {
		return err
	}
	conf.BufR = bufio.NewReader(conf.Conn)
	
	dtd := models.DTD{}
	err = tplPost(tplInit, "", &dtd)
	if err != nil {
		log.Error("auth tplInit err ", err)
		return err
	}
	
	conf.Prof.AuthPath = dtd.Auth.Form.Action
	conf.Prof.MacAddress = conf.LocalInterface.Mac
	conf.Prof.AppVersion = conf.Cfg.AgentVersion
	
	gc := len(dtd.Auth.Form.Groups)
	if gc == 1 {
		// 适用于 group 参数为空，但服务端有唯一用户组的情况，重写 Prof.Group
		conf.Prof.Group = dtd.Auth.Form.Groups[0]
	} else if gc > 1 {
		if !network.InArray(dtd.Auth.Form.Groups, conf.Prof.Group) {
			return fmt.Errorf("group error, available user groups are: %s", strings.Join(dtd.Auth.Form.Groups, " "))
		}
	}
	
	return nil
}

// PasswordAuth 认证成功后，服务端新建 ConnSession，并生成 SessionToken 或者通过 Header 返回 WebVpnCookie
func PasswordAuth() error {
	dtd := models.DTD{}
	// 发送用户名或者用户名+密码
	err := tplPost(tplAuthReply, conf.Prof.AuthPath, &dtd)
	if err != nil {
		return err
	}
	// 兼容两步登陆，如必要则再次发送
	if dtd.Type == "auth-request" && dtd.Auth.Error.Value == "" {
		dtd = models.DTD{}
		err = tplPost(tplAuthReply, conf.Prof.AuthPath, &dtd)
		if err != nil {
			return err
		}
	}
	// 用户名、密码等错误
	if dtd.Type == "auth-request" {
		if dtd.Auth.Error.Value != "" {
			return fmt.Errorf(dtd.Auth.Error.Value, dtd.Auth.Error.Param1)
		}
		return errors.New(dtd.Auth.Message)
	}
	conf.Prof.User = dtd.User
	// AnyConnect 客户端支持 XML，OpenConnect 不使用 XML，而是使用 Cookie 反馈给客户端登陆状态
	session.Sess.SessionToken = dtd.SessionToken
	// 兼容 OpenConnect
	if WebVpnCookie != "" {
		session.Sess.SessionToken = WebVpnCookie
	}
	log.Debug("SessionToken=" + session.Sess.SessionToken)
	return nil
}

// 渲染模板并发送请求
func tplPost(typ int, path string, dtd *models.DTD) error {
	var tplBuffer bytes.Buffer
	if typ == tplInit {
		t, _ := template.New("init").Parse(templateInit)
		_ = t.Execute(&tplBuffer, conf.Prof)
	} else {
		t, _ := template.New("auth_reply").Parse(templateAuthReply)
		_ = t.Execute(&tplBuffer, conf.Prof)
	}
	log.Info("tplPost url:", conf.Prof.Scheme+conf.Prof.HostWithPort+path)
	req, err := http.NewRequest("POST", conf.Prof.Scheme+conf.Prof.HostWithPort+path, &tplBuffer)
	if err != nil {
		log.Error("tplPost url:", conf.Prof.Scheme+conf.Prof.HostWithPort+path, err)
		return err
	}
	req.Header = conf.Prof.HeaderParam
	
	err = req.Write(conf.Conn)
	if err != nil {
		conf.Conn.Close()
		return err
	}
	
	var resp *http.Response
	resp, err = http.ReadResponse(conf.BufR, req)
	if err != nil {
		conf.Conn.Close()
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			conf.Conn.Close()
			return err
		}
		log.Debug(string(body))
		err = xml.Unmarshal(body, dtd)
		if dtd.Type == "complete" {
			// 兼容 ocserv
			cookies := resp.Cookies()
			if len(cookies) != 0 {
				for _, c := range cookies {
					if c.Name == "webvpn" {
						WebVpnCookie = c.Value
						break
					}
				}
			}
		}
		// nil
		return err
	}
	conf.Conn.Close()
	return fmt.Errorf("auth error %s", resp.Status)
}

var templateInit = `<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="init" aggregate-auth-version="2">
    <version who="vpn">{{.AppVersion}}</version>
</config-auth>`

// https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-03#section-2.1.2.2
var templateAuthReply = `<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="auth-reply" aggregate-auth-version="2">
    <version who="vpn">{{.AppVersion}}</version>
    <mac-address-list>
        <mac-address public-interface="true">{{.MacAddress}}</mac-address>
    </mac-address-list>
    <auth>
        <username>{{.Username}}</username>
        <password>{{.Password}}</password>
    </auth>
    <group-select>{{.Group}}</group-select>
</config-auth>`
