package utils

import (
    "errors"
    "fmt"
    "net"
    "os/exec"
    "vpnagent/base"
)

func ConfigInterface(TunName, VPNAddress, VPNMask string, DNS []string) error {
    return nil
}

func SetRoutes(ServerIP string, SplitInclude, SplitExclude *[]string) error {
    // 设置默认路由
    execCmd([]string{"route", "add", "default", ServerIP})
    execCmd([]string{"route", "change", "default", ServerIP})
    return nil
}

func ResetRoutes(ServerIP string, DNS, SplitExclude []string) {
    execCmd([]string{"route", "add", "default", base.LocalInterface.Gateway})
    execCmd([]string{"route", "change", "default", base.LocalInterface.Gateway})
}

func GetLocalInterface() error {
    interfaces := getAllInterfaces()
    if len(interfaces) < 0 {
        return errors.New("no interface found")
    }
    primaryInterface := interfaces[0]
    base.LocalInterface.Name = primaryInterface.Name
    base.LocalInterface.Mac = primaryInterface.HardwareAddr
    base.LocalInterface.Ip4 = getInterfaceIpv4(primaryInterface)
    return nil
}

func routingError(dst *net.IPNet) error {
    return fmt.Errorf("routing error: %s", dst.String())
}

func execCmd(cmdStrs []string) error {
    for _, cmdStr := range cmdStrs {
        cmd := exec.Command("sh", "-c", cmdStr)
        b, err := cmd.CombinedOutput()
        if err != nil {
            return fmt.Errorf("%s %s", string(b), cmd.String())
        }
    }
    return nil
}

func getDefaultGateway() (string, error) {
    // execCmd("route -n get default |grep gateway |awk '{print $2}'")
    gateway, err := exec.Command("route", "-n", "get", "default", "|", "grep", "gateway", "|", "awk", "'{print", "$2}'").Output()
    if err != nil {
        return "", errors.New("get default interface gateway failed: " + err.Error())
    }
    return string(gateway), nil
}

func getAllInterfaces() []net.Interface {
    iFaceList, err := net.Interfaces()
    if err != nil {
        log.Println(err)
        return nil
    }

    var outInterfaces []net.Interface
    for _, iFace := range iFaceList {
        if iFace.Flags&net.FlagLoopback == 0 && iFace.Flags&net.FlagUp == 1 && isPhysicalInterface(iFace.Name) {
            netAddrList, _ := iFace.Addrs()
            if len(netAddrList) > 0 {
                outInterfaces = append(outInterfaces, iFace)
            }
        }
    }
    return outInterfaces
}

// isPhysicalInterface returns true if the interface is physical
func isPhysicalInterface(addr string) bool {
    prefixArray := []string{"ens", "enp", "enx", "eno", "eth", "en0", "wlan", "wlp", "wlo", "wlx", "wifi0", "lan0"}
    for _, pref := range prefixArray {
        if strings.HasPrefix(strings.ToLower(addr), pref) {
            return true
        }
    }
    return false
}

func getInterfaceIpv4(p net.Interface) string {
    netAddrs, _ := p.Addrs()
    for _, addr := range netAddrs {
        ip, ok := addr.(*net.IPNet)
        if ok && ip.IP.To4() != nil && !ip.IP.IsLoopback() {
            return ip.IP.String()
        }
    }
    return ""
}
