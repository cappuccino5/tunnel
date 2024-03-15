package network

import (
	"encoding/json"
	"fmt"
	"github.com/kelleygo/trojan-go/log"
	"github.com/vishvananda/netlink"
	"net"
	"os/exec"
	"proxy/tunnel/config"
)

var (
	localInterface netlink.Link
	iface          netlink.Link
)

func ConfigInterface(TunName, VPNAddress, VPNMask string, DNS []string) error {
	var err error
	iface, err = netlink.LinkByName(TunName)
	if err != nil {
		return err
	}
	// ip address
	
	_ = netlink.LinkSetUp(iface)
	_ = netlink.LinkSetMulticastOff(iface)
	
	addr, _ := netlink.ParseAddr(IpMask2CIDR(VPNAddress, VPNMask))
	
	ifaceB, _ := json.Marshal(iface.Attrs())
	log.Info("ConfigInterface:", addr, " iface ", string(ifaceB))
	err = netlink.AddrAdd(iface, addr)
	if err != nil {
		return err
	}
	
	// dns
	if len(DNS) > 0 {
		CopyFile("/tmp/resolv.conf.bak", "/etc/resolv.conf")
		var dnsString string
		for _, dns := range DNS {
			dnsString += fmt.Sprintf("nameserver %s\n", dns)
		}
		NewRecord("/etc/resolv.conf").Prepend(dnsString)
		// time.Sleep(time.Duration(6) * time.Second)
	}
	
	return err
}

// linux设置路由的时候需要删除默认的路由，因为网络默认只能走一条路由规则
func delDefaultRoutes() error {
	localInterfaceIndex := localInterface.Attrs().Index
	dst, _ := netlink.ParseIPNet(IpMaskToCIDR("0.0.0.0/0.0.0.0"))
	gateway := net.ParseIP(config.LocalInterface.Gateway)
	route := netlink.Route{LinkIndex: localInterfaceIndex, Dst: dst, Gw: gateway}
	err := netlink.RouteDel(&route)
	if err != nil {
		log.Error("delete default route fail", err)
		return routingError(dst)
	}
	return nil
}

// 程序退出时需要恢复默认网络路由，否则与原来默认路由无法通信
func addDefaultRoutes() error {
	localInterfaceIndex := localInterface.Attrs().Index
	dst, _ := netlink.ParseIPNet(IpMaskToCIDR("0.0.0.0/0.0.0.0"))
	gateway := net.ParseIP(config.LocalInterface.Gateway)
	route := netlink.Route{LinkIndex: localInterfaceIndex, Dst: dst, Gw: gateway}
	err := netlink.RouteAdd(&route)
	if err != nil {
		return err
	}
	return nil
}

func SetRoutes(ServerIP string, SplitInclude, SplitExclude *[]string) error {
	err := delDefaultRoutes()
	if err != nil {
		return fmt.Errorf("delDefaultRoutes error: ", err.Error())
	}
	//set new routes
	localInterfaceIndex := localInterface.Attrs().Index
	dst, _ := netlink.ParseIPNet(ServerIP + "/32")
	gateway := net.ParseIP(config.LocalInterface.Gateway)
	ifaceIndex := iface.Attrs().Index
	
	route := netlink.Route{LinkIndex: localInterfaceIndex, Dst: dst, Gw: gateway}
	err = netlink.RouteAdd(&route)
	if err != nil {
		return routingError(dst)
	}
	
	if len(*SplitInclude) == 0 {
		*SplitInclude = append(*SplitInclude, "0.0.0.0/0.0.0.0")
	}
	for _, ipMask := range *SplitInclude {
		dst, _ = netlink.ParseIPNet(IpMaskToCIDR(ipMask))
		route = netlink.Route{LinkIndex: ifaceIndex, Dst: dst, Priority: 1}
		err = netlink.RouteAdd(&route)
		if err != nil {
			return routingError(dst)
		}
	}
	
	// 支持在 SplitInclude 网段中排除某个路由
	if len(*SplitExclude) > 0 {
		for _, ipMask := range *SplitExclude {
			dst, _ = netlink.ParseIPNet(IpMaskToCIDR(ipMask))
			route = netlink.Route{LinkIndex: localInterfaceIndex, Dst: dst, Gw: gateway, Priority: 5}
			err = netlink.RouteAdd(&route)
			if err != nil {
				return routingError(dst)
			}
		}
	}
	
	return err
}

func ResetRoutes(ServerIP string, DNS, SplitExclude []string) {
	// routes
	localInterfaceIndex := localInterface.Attrs().Index
	dst, _ := netlink.ParseIPNet(ServerIP + "/32")
	log.Info("ResetRoutes DEL------------>", localInterfaceIndex, "  ", dst)
	_ = netlink.RouteDel(&netlink.Route{LinkIndex: localInterfaceIndex, Dst: dst})
	
	if len(SplitExclude) > 0 {
		for _, ipMask := range SplitExclude {
			dst, _ = netlink.ParseIPNet(IpMaskToCIDR(ipMask))
			log.Info("ResetRoutes DEL------------>", localInterfaceIndex, "  ", dst)
			_ = netlink.RouteDel(&netlink.Route{LinkIndex: localInterfaceIndex, Dst: dst})
		}
	}
	log.Info("ResetRoutes", localInterface.Attrs().Name, config.LocalInterface.Gateway, config.LocalInterface.Ip4)
	err := addDefaultRoutes()
	if err != nil {
		log.Error("ResetDefaultRoutes failed", err)
	}
	// dns
	if len(DNS) > 0 {
		CopyFile("/etc/resolv.conf", "/tmp/resolv.conf.bak")
	}
}

func GetLocalInterface() error {
	
	// just for default route
	routes, err := netlink.RouteGet(net.ParseIP("8.8.8.8"))
	if len(routes) > 0 {
		route := routes[0]
		localInterface, err = netlink.LinkByIndex(route.LinkIndex)
		if err != nil {
			return err
		}
		config.LocalInterface.Name = localInterface.Attrs().Name
		config.LocalInterface.Ip4 = route.Src.String()
		config.LocalInterface.Gateway = route.Gw.String()
		config.LocalInterface.Mac = localInterface.Attrs().HardwareAddr.String()
		
		log.Info("GetLocalInterface: ", *config.LocalInterface)
		return nil
	}
	return err
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
