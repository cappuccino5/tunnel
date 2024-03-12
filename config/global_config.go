package config

import (
	"bufio"
	"net"
	"net/http"
	"proxy/tunnel/models"
)

var (
	Cfg            = &ClientConfig{}
	LocalInterface = &Interface{}
)

type ClientConfig struct {
	LogLevel           string `json:"log_level"`
	LogPath            string `json:"log_path"`
	InsecureSkipVerify bool   `json:"skip_verify"`
	CiscoCompat        bool   `json:"cisco_compat"`
	AgentName          string `json:"agent_name"`
	AgentVersion       string `json:"agent_version"`
}

func init() {
	//Cfg.LogLevel = "Info"
	//Cfg.LogPath = "./logs"
	Cfg.InsecureSkipVerify = true
	Cfg.CiscoCompat = true
	Cfg.AgentName = "edge vpn Client"
	Cfg.AgentVersion = "0.2.0.6"
}

// auth 里初始化
var (
	Prof = &Profile{Initialized: false}
	Conn net.Conn
	BufR *bufio.Reader
)

// Profile 模板变量字段必须导出，虽然全局，但每次连接都被重置
type Profile struct {
	Host       string `json:"host"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Group      string `json:"group"`
	MacAddress string
	
	HostWithPort string
	Scheme       string
	AuthPath     string
	
	User        models.User
	HeaderParam http.Header
	AppVersion  string // for report to server in xml
	Initialized bool
}

// Interface 应该由外部接口设置
type Interface struct {
	Name    string `json:"name"`
	Ip4     string `json:"ip4"`
	Mac     string `json:"mac"`
	Gateway string `json:"gateway"`
}
