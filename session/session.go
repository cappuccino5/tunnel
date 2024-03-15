package session

import (
	"github.com/kelleygo/trojan-go/log"
	"net/http"
	"proxy/tunnel/config"
	"proxy/tunnel/models"
	utils "proxy/tunnel/network"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	Sess = &Session{}
)

type Session struct {
	SessionToken    string
	PreMasterSecret []byte
	
	Connected   bool
	ActiveClose bool
	CloseChan   chan struct{} // 用于监听 TLS 通道是否关闭
	CSess       *ConnSession
}

type stat struct {
	// be sure to use the double type when parsing
	BytesSent     uint64 `json:"bytesSent"`
	BytesReceived uint64 `json:"bytesReceived"`
}

// ConnSession used for both TLS and DTLS
type ConnSession struct {
	ServerAddress     string
	LocalAddress      string
	Hostname          string
	TunName           string
	VPNAddress        string // The IPv4 address of the client
	VPNMask           string // IPv4 netmask
	DNS               []string
	MTU               int
	SplitInclude      []string
	SplitExclude      []string
	TLSCipherSuite    string
	TLSDpdTime        int // https://datatracker.ietf.org/doc/html/rfc3706
	TLSKeepaliveTime  int
	DTLSPort          string
	DTLSDpdTime       int
	DTLSKeepaliveTime int
	DTLSId            string `json:"-"` // used by the server to associate the DTLS channel with the CSTP channel
	DTLSCipherSuite   string
	Stat              *stat
	
	closeOnce      sync.Once            `json:"-"`
	CloseChan      chan struct{}        `json:"-"`
	PayloadIn      chan *models.Payload `json:"-"`
	PayloadOutTLS  chan *models.Payload `json:"-"`
	PayloadOutDTLS chan *models.Payload `json:"-"`
	
	DtlsConnected bool
	DtlsSetupChan chan struct{} `json:"-"`
	DtlsSession   *DtlsSession  `json:"-"`
	
	ResetTLSReadDead  atomic.Value `json:"-"`
	ResetDTLSReadDead atomic.Value `json:"-"`
}

type DtlsSession struct {
	closeOnce sync.Once
	CloseChan chan struct{}
}

func (sess *Session) NewConnSession(header *http.Header) *ConnSession {
	cSess := &ConnSession{
		LocalAddress:   config.LocalInterface.Ip4,
		Stat:           &stat{0, 0},
		closeOnce:      sync.Once{},
		CloseChan:      make(chan struct{}),
		DtlsSetupChan:  make(chan struct{}),
		PayloadIn:      make(chan *models.Payload, 64),
		PayloadOutTLS:  make(chan *models.Payload, 64),
		PayloadOutDTLS: make(chan *models.Payload, 64),
	}
	cSess.ResetTLSReadDead.Store(true) // 初始化读取超时定时器
	sess.CSess = cSess
	
	sess.Connected = true
	sess.ActiveClose = false
	sess.CloseChan = make(chan struct{})
	
	cSess.VPNAddress = header.Get("X-CSTP-Address")
	//cSess.VPNAddress = "0.0.0.0"
	cSess.VPNMask = header.Get("X-CSTP-Netmask")
	//cSess.VPNMask = "255.255.254.0"
	cSess.MTU, _ = strconv.Atoi(header.Get("X-CSTP-MTU"))
	cSess.DNS = header.Values("X-CSTP-DNS")
	cSess.SplitInclude = header.Values("X-CSTP-Split-Include")
	cSess.SplitExclude = header.Values("X-CSTP-Split-Exclude")
	cSess.TLSDpdTime, _ = strconv.Atoi(header.Get("X-CSTP-DPD"))
	cSess.TLSKeepaliveTime, _ = strconv.Atoi(header.Get("X-CSTP-Keepalive"))
	// https://datatracker.ietf.org/doc/html/draft-mavrogiannopoulos-openconnect-02#section-2.1.5.1
	cSess.DTLSId = header.Get("X-DTLS-Session-ID")
	if cSess.DTLSId == "" {
		// 兼容最新 ocserv
		cSess.DTLSId = header.Get("X-DTLS-App-ID")
	}
	cSess.DTLSPort = header.Get("X-DTLS-Port")
	// dtls.ConnectionState 没有直接暴露出相关信息
	cSess.DTLSCipherSuite = header.Get("X-DTLS12-CipherSuite")
	cSess.DTLSDpdTime, _ = strconv.Atoi(header.Get("X-DTLS-DPD"))
	cSess.DTLSKeepaliveTime, _ = strconv.Atoi(header.Get("X-DTLS-Keepalive"))
	
	return cSess
}

func (cSess *ConnSession) NewDtlsSession() *DtlsSession {
	cSess.DtlsSession = &DtlsSession{
		closeOnce: sync.Once{},
		CloseChan: make(chan struct{}),
	}
	cSess.ResetDTLSReadDead.Store(true)
	cSess.DtlsConnected = true
	return cSess.DtlsSession
}

func (cSess *ConnSession) DPDTimer() {
	go func() {
		defer func() {
			log.Info("dead peer detection timer exit")
		}()
		log.Debug("TLSDpdTime:", cSess.TLSDpdTime, "TLSKeepaliveTime", cSess.TLSKeepaliveTime,
			"DTLSDpdTime", cSess.DTLSDpdTime, "DTLSKeepaliveTime", cSess.DTLSKeepaliveTime)
		// 简化处理，最小15秒检测一次,至少5秒冗余
		dpdTime := utils.Min(cSess.TLSDpdTime, cSess.DTLSDpdTime) - 5
		if dpdTime < 10 {
			dpdTime = 10
		}
		tick := time.NewTicker(time.Duration(dpdTime) * time.Second)
		
		tlsDpd := models.Payload{
			PType: 0x03,
			Data:  make([]byte, 0, 8),
		}
		dtlsDpd := models.Payload{
			PType: 0x03,
			Data:  make([]byte, 0, 1),
		}
		
		for {
			select {
			case <-tick.C:
				// base.Debug("dead peer detection")
				select {
				case cSess.PayloadOutTLS <- &tlsDpd:
				default:
				}
				if cSess.DtlsSession != nil {
					select {
					case cSess.PayloadOutDTLS <- &dtlsDpd:
					default:
					}
				}
			case <-cSess.CloseChan:
				tick.Stop()
				return
			}
		}
	}()
}

func (cSess *ConnSession) ReadDeadTimer() {
	go func() {
		defer func() {
			log.Info("read dead timer exit")
		}()
		// 避免每次 for 循环都重置读超时的时间
		// 这里是绝对时间，至于链接本身，服务器没有数据时 conn.Read 会阻塞，有数据时会不断判断
		tick := time.NewTicker(4 * time.Second)
		for range tick.C {
			select {
			case <-cSess.CloseChan:
				tick.Stop()
				return
			default:
				cSess.ResetTLSReadDead.Store(true)
				cSess.ResetDTLSReadDead.Store(true)
			}
		}
	}()
}

func (cSess *ConnSession) Close() {
	cSess.closeOnce.Do(func() {
		if cSess.DtlsSession != nil {
			cSess.DtlsSession.Close()
		}
		close(cSess.CloseChan)
		utils.ResetRoutes(cSess.ServerAddress, cSess.DNS, cSess.SplitExclude)
		Sess.CSess = nil
		Sess.Connected = false
		
		close(Sess.CloseChan)
	})
}

func (dSess *DtlsSession) Close() {
	dSess.closeOnce.Do(func() {
		close(dSess.CloseChan)
		if Sess.CSess != nil {
			Sess.CSess.DtlsConnected = false
			Sess.CSess.DtlsSession = nil
		}
	})
}
