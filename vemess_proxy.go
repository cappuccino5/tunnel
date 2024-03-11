package main

import (
	"bufio"
	"dev.risinghf.com/go/framework/log"
	"encoding/hex"
	"net"
	"time"
)

// 复用已有的 tls.Conn 和对应的 bufR
func tlsVmessChannel(conn net.Conn, bufR *bufio.Reader, cSess *ConnSession) {
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
		//if cSess.ResetReadDead.Load().(bool) {
		//	cSess.ResetReadDead.Store(false)
		//}

		pl := getPayloadBuffer()                // 从池子申请一块内存，存放去除头部的数据包到 PayloadIn，在 payloadInToTun 中释放
		bytesReceived, err = bufR.Read(pl.Data) // 服务器没有数据返回时，会阻塞
		if err != nil {
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
func payloadOutTLSToServer(conn net.Conn, cSess *ConnSession) {
	defer func() {
		log.Info("tls payloadOut to server exit")
		_ = conn.Close()
		cSess.Close()
	}()

	var (
		err       error
		bytesSent int
		pl        *Payload
	)

	for {
		select {
		case pl = <-cSess.PayloadOut:
		case <-cSess.CloseChan:
			return
		}

		// base.Debug("tls payloadOut to server", "PType", pl.PType)
		log.Info("payloadOutTLSToServer ", hex.EncodeToString(pl.Data))
		// TODO
		if conn == nil {
			continue
		}
		bytesSent, err = conn.Write(pl.Data)
		if err != nil {
			log.Error("tls payloadOut to server error:", err)
			return
		}
		cSess.Stat.BytesSent += uint64(bytesSent)

		// 释放由 tunToPayloadOut 申请的内存
		putPayloadBuffer(pl)
	}
}
