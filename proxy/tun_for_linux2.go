package vpn

import (
	"dev.risinghf.com/go/framework/log"
	"encoding/hex"
	"fmt"
	"github.com/songgao/water"
	"net/netip"
	"os/exec"
	"proxy/tunnel/config"
	"proxy/tunnel/models"
	"proxy/tunnel/session"
)

// 创建tun网卡
func LinkTun(cSess *session.ConnSession) error {
	cfg := water.Config{
		DeviceType: water.TUN,
	}
	
	ifce, err := water.New(cfg)
	if err != nil {
		return err
	}
	// log.Printf("Interface Name: %s\n", ifce.Name())
	//cSess.SetIfName(ifce.Name())
	// routes
	dst, err := netip.ParsePrefix(cSess.ServerAddress + "/32")
	nextHopVPNGateway, _ := netip.ParseAddr(config.LocalInterface.Gateway)
	cmdstr1 := fmt.Sprintf("ip link set dev %s up mtu %d multicast off", ifce.Name(), cSess.MTU)
	cmdstr2 := fmt.Sprintf("ip addr add dev %s local %s peer %s",
		ifce.Name(), nextHopVPNGateway, dst)
	log.Debug("---------------> LinkTun:", cmdstr1, cmdstr2)
	err = execCmd([]string{cmdstr1, cmdstr2})
	if err != nil {
		log.Error(err)
		_ = ifce.Close()
		return err
	}
	
	cmdstr3 := fmt.Sprintf("sysctl -w net.ipv6.conf.%s.disable_ipv6=1", ifce.Name())
	execCmd([]string{cmdstr3})
	
	go tunRead(ifce, cSess)
	go tunWrite(ifce, cSess)
	return nil
}

func tunWrite(ifce *water.Interface, cSess *session.ConnSession) {
	defer func() {
		log.Debug("LinkTun return", cSess.VPNAddress)
		_ = ifce.Close()
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
		var tempData []byte = make([]byte, len(pl.Data))
		copy(tempData, pl.Data)
		log.Debug(pl.PType, "--------->linux tunWrite ", hex.EncodeToString(tempData))
		
		_, err = ifce.Write(pl.Data)
		if err != nil {
			log.Error("tun Write err", err)
			return
		}
		
		models.PutPayloadBuffer(pl)
	}
}

func tunRead(ifce *water.Interface, cSess *session.ConnSession) {
	defer func() {
		log.Debug("tunRead return", cSess.VPNAddress)
		_ = ifce.Close()
	}()
	var (
		err error
		n   int
	)
	
	for {
		// data := make([]byte, BufferSize)
		pl := models.GetPayloadBuffer()
		n, err = ifce.Read(pl.Data)
		if err != nil {
			log.Error("tun Read err", n, err)
			return
		}
		log.Info("------------------>linux tunRead size: ", n)
		// 更新数据长度
		pl.Data = (pl.Data)[:n]
		
		// data = data[:n]
		// ip_src := waterutil.IPv4Source(data)
		// ip_dst := waterutil.IPv4Destination(data)
		// ip_port := waterutil.IPv4DestinationPort(data)
		// fmt.Println("sent:", ip_src, ip_dst, ip_port)
		// packet := gopacket.NewPacket(data, layers.LayerTypeIPv4, gopacket.Default)
		// fmt.Println("read:", packet)
		
		if payloadOutCstp(cSess, pl) {
			return
		}
	}
}

func payloadOutCstp(cSess *session.ConnSession, pl *models.Payload) bool {
	closed := false
	
	select {
	case cSess.PayloadOutTLS <- pl:
	case <-cSess.CloseChan:
		closed = true
	}
	
	return closed
}

func execCmd(cmdStrs []string) error {
	for _, cmdStr := range cmdStrs {
		cmd := exec.Command("sh", "-c", cmdStr)
		b, err := cmd.CombinedOutput()
		if err != nil {
			log.Info("execCmd:", string(b))
			return err
		}
	}
	return nil
}
