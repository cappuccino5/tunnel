package vpn

import (
	"bytes"
	"dev.risinghf.com/go/framework/log"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"proxy/tunnel/config"
	"proxy/tunnel/network"
	"proxy/tunnel/session"
	"strings"
)

var (
	reqHeaders = make(map[string]string)
)

func init() {
	reqHeaders["X-CSTP-HostWithPort"], _ = os.Hostname()
	reqHeaders["X-CSTP-VPNAddress-Type"] = "IPv4"
	// Payload + 8 + 加密扩展位 + TCP或UDP头 + IP头 最好小于 1500，这里参考 AnyConnect 设置
	reqHeaders["X-CSTP-MTU"] = "1399"
	reqHeaders["X-CSTP-Base-MTU"] = "1399"
	// if base.Cfg.OS == "android" || base.Cfg.OS == "ios" {
	//    reqHeaders["X-CSTP-License"] = "mobile"
	// }
}

func initTunnel() {
	// https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-03#section-2.1.3
	reqHeaders["Cookie"] = "webvpn=" + session.Sess.SessionToken // 无论什么服务端都需要通过 Cookie 发送 Session
	reqHeaders["X-CSTP-Local-VPNAddress-IP4"] = config.LocalInterface.Ip4
	
	// Legacy Establishment of Secondary UDP Channel https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-02#section-2.1.5.1
	// worker-vpn.c WSPCONFIG(ws)->udp_port != 0 && req->master_secret_set != 0 否则 disabling UDP (DTLS) connection
	// 如果开启 dtls_psk（默认开启，见配置说明） 且 CipherSuite 包含 PSK-NEGOTIATE（仅限ocserv），worker-http.c 自动设置 req->master_secret_set = 1
	// 此时无需手动设置 Secret，会自动协商建立 dtls 链接，AnyConnect 客户端不支持
	session.Sess.PreMasterSecret, _ = network.MakeMasterSecret()
	reqHeaders["X-DTLS-Master-Secret"] = hex.EncodeToString(session.Sess.PreMasterSecret) // A hex encoded pre-master secret to be used in the legacy DTLS session negotiation
	
	// https://gitlab.com/openconnect/ocserv/-/blob/master/src/worker-http.c#L150
	// https://github.com/openconnect/openconnect/blob/master/gnutls-dtls.c#L75
	// worker-http.c dtls_ciphersuite_st ciphersuites12[]   仅支持两种，AES128-GCM-SHA256 和 AES256-GCM-SHA384
	// AES128-GCM-SHA256 返回的 ID 为 0x009c，因此判断为 0x00,0x9C	TLS_RSA_WITH_AES_128_GCM_SHA256
	// https://www.rfc-editor.org/rfc/rfc5288.html or https://www.iana.org/assignments/tls-parameters/tls-parameters.xml#tls-parameters-4
	// 冒号分隔，对于 ocserv，如果设置 PSK-NEGOTIATE，这里无效，我们目前不支持 PSK
	reqHeaders["X-DTLS12-CipherSuite"] = "ECDHE-ECDSA-AES128-GCM-SHA256:AES128-GCM-SHA256"
}

// SetupTunnel initiates an HTTP CONNECT command to establish a VPN
func SetupTunnel() error {
	initTunnel()
	
	// https://github.com/golang/go/commit/da6c168378b4c1deb2a731356f1f438e4723b8a7
	// https://github.com/golang/go/issues/17227#issuecomment-341855744
	req, _ := http.NewRequest("CONNECT", config.Prof.Scheme+config.Prof.HostWithPort+"/CSCOSSLC/tunnel", nil)
	req.Header = config.Prof.HeaderParam
	for k, v := range reqHeaders {
		// req.Header.Set 会将首字母大写，其它小写
		req.Header[k] = []string{v}
	}
	
	// 发送 CONNECT 请求
	err := req.Write(config.Conn)
	if err != nil {
		config.Conn.Close()
		return err
	}
	var resp *http.Response
	// resp.Body closed when tlsChannel exit
	resp, err = http.ReadResponse(config.BufR, req)
	if err != nil {
		config.Conn.Close()
		return err
	}
	
	if resp.StatusCode != http.StatusOK {
		config.Conn.Close()
		return fmt.Errorf("tunnel negotiation failed %s", resp.Status)
	}
	// 协商成功，读取服务端返回的配置
	// https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-03#section-2.1.3
	
	// 提前判断是否调试模式，避免不必要的转换，http.ReadResponse.Header 将首字母大写，其余小写，即使服务端调试时正常
	if log.GetLogLevel() == "debug" {
		headers := make([]byte, 0)
		buf := bytes.NewBuffer(headers)
		// http.ReadResponse: Keys in the map are canonicalized (see CanonicalHeaderKey).
		// https://ron-liu.medium.com/what-canonical-http-header-mean-in-golang-2e97f854316d
		_ = resp.Header.Write(buf)
		log.Debug(buf.String())
	}
	
	cSess := session.Sess.NewConnSession(&resp.Header)
	session.Sess.CSess = cSess
	cSess.ServerAddress = strings.Split(config.Conn.RemoteAddr().String(), ":")[0]
	cSess.Hostname = config.Prof.Host
	// cSess.TLSCipherSuite = tls.CipherSuiteName(auth.Conn.ConnectionState().CipherSuite)
	
	tempB, _ := json.Marshal(cSess)
	log.Info("----------> session:", string(tempB))
	
	err = setupTun(cSess)
	if err != nil {
		config.Conn.Close()
		cSess.Close()
		return err
	}
	
	cSess.DPDTimer()
	cSess.ReadDeadTimer()
	
	// 为了靠谱，不再异步设置，路由多的话可能要等等
	err = network.SetRoutes(cSess.ServerAddress, &cSess.SplitInclude, &cSess.SplitExclude)
	if err != nil {
		config.Conn.Close()
		cSess.Close()
		return err
	}
	
	log.Info("tls channel negotiation succeeded")
	// 只有网卡和路由设置成功才会进行下一步
	// https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-03#section-2.1.4
	go tlsChannel(config.Conn, config.BufR, cSess, resp)
	if cSess.DTLSPort != "" {
		// https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-03#section-2.1.5
		//go dtlsChannel(cSess)
	}
	return err
}

func SetupSystemProxy() error {
	initTunnel()
	
	// https://github.com/golang/go/commit/da6c168378b4c1deb2a731356f1f438e4723b8a7
	// https://github.com/golang/go/issues/17227#issuecomment-341855744
	req, _ := http.NewRequest("CONNECT", config.Prof.Scheme+config.Prof.HostWithPort+"/CSCOSSLC/tunnel", nil)
	req.Header = config.Prof.HeaderParam
	for k, v := range reqHeaders {
		// req.Header.Set 会将首字母大写，其它小写
		req.Header[k] = []string{v}
	}
	
	// 发送 CONNECT 请求
	err := req.Write(config.Conn)
	if err != nil {
		config.Conn.Close()
		return err
	}
	var resp *http.Response
	// resp.Body closed when tlsChannel exit
	resp, err = http.ReadResponse(config.BufR, req)
	if err != nil {
		config.Conn.Close()
		return err
	}
	
	if resp.StatusCode != http.StatusOK {
		config.Conn.Close()
		return fmt.Errorf("tunnel negotiation failed %s", resp.Status)
	}
	// 协商成功，读取服务端返回的配置
	// https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-03#section-2.1.3
	
	// 提前判断是否调试模式，避免不必要的转换，http.ReadResponse.Header 将首字母大写，其余小写，即使服务端调试时正常
	if log.GetLogLevel() == "debug" {
		headers := make([]byte, 0)
		buf := bytes.NewBuffer(headers)
		// http.ReadResponse: Keys in the map are canonicalized (see CanonicalHeaderKey).
		// https://ron-liu.medium.com/what-canonical-http-header-mean-in-golang-2e97f854316d
		_ = resp.Header.Write(buf)
		log.Debug(buf.String())
	}
	
	cSess := session.Sess.NewConnSession(&resp.Header)
	cSess.ServerAddress = strings.Split(config.Conn.RemoteAddr().String(), ":")[0]
	cSess.Hostname = config.Prof.Host
	// cSess.TLSCipherSuite = tls.CipherSuiteName(config.Conn.ConnectionState().CipherSuite)
	
	log.Info("tls channel negotiation succeeded")
	// 只有网卡设置成功才会进行下一步
	// https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-03#section-2.1.4
	go tlsChannel(config.Conn, config.BufR, cSess, resp)
	
	return nil
}
