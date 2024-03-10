package main

import (
	"flag"
	"log"
	"os"
)

func main() {

	flagSet := flag.NewFlagSet("project-start", flag.ExitOnError)
	testRun := flagSet.String("test", "", "测试代理链路是否正常")
	// 打印
	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%s", *testRun)
	SetupTunnel()
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

	err = setupTun(cSess)
	if err != nil {
		auth.Conn.Close()
		cSess.Close()
		return err
	}
	base.Info("tls channel negotiation succeeded")
	// 只有网卡设置成功才会进行下一步
	// https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-03#section-2.1.4
	go tlsChannel(auth.Conn, auth.BufR, cSess, resp)
	if cSess.DTLSPort != "" {
		// https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-03#section-2.1.5
		go dtlsChannel(cSess)
	}
	cSess.DPDTimer()
	cSess.ReadDeadTimer()

	//为了靠谱，不再异步设置，路由多的话可能要等等
	//err = utils.SetRoutes(cSess.ServerAddress, &cSess.SplitInclude, &cSess.SplitExclude)
	//if err != nil {
	//	auth.Conn.Close()
	//	cSess.Close()
	//}

	return nil
}
