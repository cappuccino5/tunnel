package session

import (
	"dev.risinghf.com/go/framework/log"
	"os"
	"proxy/tunnel/config"
	"proxy/tunnel/models"
	"proxy/tunnel/network"
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
	ServerAddress string
	LocalAddress  string
	Hostname      string
	TunName       string
	VPNAddress    string // The IPv4 address of the client
	VPNMask       string // IPv4 netmask
	DNS           []string
	
	MTU           int
	Stat          *stat
	closeOnce     sync.Once            `json:"-"`
	CloseChan     chan struct{}        `json:"-"`
	PayloadIn     chan *models.Payload `json:"-"`
	PayloadOut    chan *models.Payload `json:"-"`
	ResetReadDead atomic.Value         `json:"-"`
}

func (sess *Session) NewConnSession() *ConnSession {
	cSess := &ConnSession{
		Stat:       &stat{0, 0},
		closeOnce:  sync.Once{},
		CloseChan:  make(chan struct{}),
		PayloadOut: make(chan *models.Payload, 64),
		PayloadIn:  make(chan *models.Payload, 64),
	}
	cSess.ResetReadDead.Store(true) // 初始化读取超时定时器
	sess.CSess = cSess
	sess.Connected = true
	sess.ActiveClose = false
	sess.CloseChan = make(chan struct{})
	
	cSess.MTU = 1399
	cSess.Hostname, _ = os.Hostname()
	return cSess
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
				cSess.ResetReadDead.Store(true)
			}
		}
	}()
}

func (cSess *ConnSession) Close() {
	cSess.closeOnce.Do(func() {
		close(cSess.CloseChan)
		network.ResetRoutes(config.Prof.ServiceAddr(), Sess.CSess.DNS, []string{})
		Sess.CSess = nil
		Sess.Connected = false
		close(Sess.CloseChan)
	})
}
