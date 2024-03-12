package proxy_client

import (
	"bufio"
	"dev.risinghf.com/go/framework/log"
	"encoding/hex"
	"io"
	"net"
	"proxy/tunnel/models"
	"proxy/tunnel/network"
	"proxy/tunnel/session"
	"proxy/tunnel/tun"
	"strings"
	"time"
)

// 复用已有的 tls.Conn 和对应的 bufR
func TlsVmessChannel(conn net.Conn, bufR *bufio.Reader, cSess *session.ConnSession) {
	defer func() {
		log.Info("tls channel exit")
		if conn != nil {
			conn.Close()
		}
		cSess.Close()
	}()
	var (
		err           error
		bytesReceived int
		dead          = time.Duration(30) * time.Second
	)
	
	go payloadOutTLSToServer(conn, cSess)
	
	// Step 21 serverToPayloadIn
	// 读取服务器返回的数据，调整格式，放入 cSess.PayloadIn，不再用子协程是为了能够退出 tlsChannel 协程
	for {
		// 重置超时限制
		if conn != nil {
			_ = conn.SetReadDeadline(time.Now().Add(dead))
		}
		if cSess.ResetReadDead.Load().(bool) {
			cSess.ResetReadDead.Store(false)
		}
		
		pl := models.GetPayloadBuffer()         // 从池子申请一块内存，存放去除头部的数据包到 PayloadIn，在 payloadInToTun 中释放
		bytesReceived, err = bufR.Read(pl.Data) // 服务器没有数据返回时，会阻塞
		if err != nil {
			if err == io.EOF {
				continue
			}
			log.Error("tls server to payloadIn error:", err)
			return
		}
		log.Debug("read service payload in:", bytesReceived, hex.EncodeToString(pl.Data))
		select {
		case cSess.PayloadIn <- pl:
		case <-cSess.CloseChan:
			return
		}
		cSess.Stat.BytesReceived += uint64(bytesReceived)
	}
}

// payloadOutTLSToServer Step 4，往代理服务器写数据
func payloadOutTLSToServer(conn net.Conn, cSess *session.ConnSession) {
	defer func() {
		log.Info("tls payloadOut to server exit")
		cSess.Close()
	}()
	
	var (
		err       error
		bytesSent int
		pl        *models.Payload
	)
	
	for {
		select {
		case pl = <-cSess.PayloadOut:
		case <-cSess.CloseChan:
			return
		}
		
		var tempData = make([]byte, len(pl.Data))
		copy(tempData, pl.Data)
		// base.Debug("tls payloadOut to server", "PType", pl.PType)
		log.Debug("payloadOutTLSToServer ", hex.EncodeToString(tempData))
		bytesSent, err = conn.Write(pl.Data)
		if err != nil {
			log.Error("tls payloadOut to server error:", err)
			return
		}
		cSess.Stat.BytesSent += uint64(bytesSent)
		
		// 释放由 tunToPayloadOut 申请的内存
		models.PutPayloadBuffer(pl)
	}
}

// Step 3
// 网络栈将应用数据包转给 tun 后，该函数从 tun 读取数据包，放入 cSess.PayloadOut
// 之后由 payloadOutTLSToServer 或 payloadOutDTLSToServer 调整格式，发送给服务端
func tunToPayloadOut(dev tun.Device, cSess *session.ConnSession) {
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
		pl := models.GetPayloadBuffer()
		n, err = dev.Read(pl.Data, 0) // 如果 tun 没有 up，会在这等待
		if err != nil {
			log.Error("tun to payloadOut error:", err)
			return
		}
		
		// 更新数据长度
		pl.Data = (pl.Data)[:n]
		
		log.Debug("read 0.0.0.0 tunToPayloadOut size:", n)
		select {
		case cSess.PayloadOut <- pl:
		case <-cSess.CloseChan:
			return
		}
	}
}

// Step 22
// 读取 tlsChannel、dtlsChannel 放入 cSess.PayloadIn 的数据包（由服务端返回，已调整格式），写入 tun，网络栈交给应用
func payloadInToTun(dev tun.Device, cSess *session.ConnSession) {
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
		pl  *models.Payload
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
		models.PutPayloadBuffer(pl)
	}
}

func SetupTun(cSess *session.ConnSession) error {
	if cSess.TunName == "" {
		cSess.TunName = "egde VPN"
	}
	cSess.TunName = strings.ToLower(cSess.TunName)
	
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
	
	/****/
	
	//nativeTunDevice := dev.(*tun.NativeTun)
	
	// 获取LUID用于配置网络
	//link := winipcfg.LUID(nativeTunDevice.LUID())
	//
	//ip, err := netip.ParsePrefix("10.0.0.77/24")
	//if err != nil {
	//	log.Error("ParsePrefix:", err)
	//	return err
	//}
	//err = link.SetIPAddresses([]netip.Prefix{ip})
	//if err != nil {
	//	log.Error("SetIPAddresses err", err)
	//	return err
	//}
	//********/
	//不可并行
	err = network.ConfigInterface(cSess.TunName, cSess.VPNAddress, cSess.VPNMask, cSess.DNS)
	if err != nil {
		_ = dev.Close()
		return err
	}
	
	go tunToPayloadOut(dev, cSess) // read from apps
	go payloadInToTun(dev, cSess)  // write to apps
	return nil
}
