package network

import (
	"fmt"
	"github.com/kelleygo/trojan-go/log"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
	"net"
	"net/netip"
	"os/exec"
	"proxy/tunnel/config"
	"proxy/tunnel/tun"
	"strings"
)

var (
	localInterface winipcfg.LUID
	iface          winipcfg.LUID
	nextHopVPN     netip.Addr
)

func SetMTU(ifname string, mtu int) error {
	cmdStr := fmt.Sprintf("netsh interface ipv4 set subinterface \"%s\" MTU=%d", ifname, mtu)
	err := execCmd([]string{cmdStr})
	return err
}

func execCmd(cmdStrs []string) error {
	for _, cmdStr := range cmdStrs {
		cmd := exec.Command("cmd", "/C", cmdStr)
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s %s", string(b), cmd.String())
		}
	}
	return nil
}

func ConfigInterface(TunName, VPNAddress, VPNMask string, DNS []string) error {
	mtu, _ := tun.NativeTunDevice.MTU()
	err := SetMTU(TunName, mtu)
	if err != nil {
		return err
	}
	
	iface = winipcfg.LUID(tun.NativeTunDevice.LUID())
	
	// ip address
	iface.FlushIPAddresses(windows.AF_UNSPEC)
	
	nextHopVPN, _ = netip.ParseAddr(VPNAddress)
	prefixVPN, _ := netip.ParsePrefix(IpMask2CIDR(VPNAddress, VPNMask))
	err = iface.SetIPAddressesForFamily(windows.AF_INET, []netip.Prefix{prefixVPN})
	if err != nil {
		return err
	}
	
	// dns
	var servers []netip.Addr
	for _, dns := range DNS {
		addr, _ := netip.ParseAddr(dns)
		servers = append(servers, addr)
	}
	
	err = iface.SetDNS(windows.AF_INET, servers, []string{})
	if err != nil {
		return err
	}
	
	return nil
}

func GetLocalInterface() error {
	ifcs, err := winipcfg.GetAdaptersAddresses(windows.AF_INET, winipcfg.GAAFlagIncludeGateways)
	if err != nil {
		return err
	}
	
	var primaryInterface *winipcfg.IPAdapterAddresses
	for _, ifc := range ifcs {
		log.Debug(ifc.AdapterName(), ifc.Description(), ifc.FriendlyName(), ifc.Ipv4Metric, ifc.IfType)
		// exclude Virtual Ethernet and Loopback Adapter
		if !strings.Contains(ifc.Description(), "Virtual") {
			// https://git.zx2c4.com/wireguard-windows/tree/tunnel/winipcfg/types.go?h=v0.5.3#n61
			if (ifc.IfType == 6 || ifc.IfType == 71) && ifc.FirstGatewayAddress != nil {
				if primaryInterface == nil || (ifc.Ipv4Metric < primaryInterface.Ipv4Metric) {
					primaryInterface = ifc
				}
			}
		}
	}
	
	log.Info("GetLocalInterface: ", primaryInterface.AdapterName(), primaryInterface.Description(),
		primaryInterface.FriendlyName(), primaryInterface.Ipv4Metric, primaryInterface.IfType)
	
	config.LocalInterface.Name = primaryInterface.FriendlyName()
	config.LocalInterface.Ip4 = primaryInterface.FirstUnicastAddress.Address.IP().String()
	config.LocalInterface.Gateway = primaryInterface.FirstGatewayAddress.Address.IP().String()
	config.LocalInterface.Mac = net.HardwareAddr(primaryInterface.PhysicalAddress()).String()
	
	localInterface = primaryInterface.LUID
	
	return nil
}

func SetRoutes(ServerIP string, SplitInclude, SplitExclude *[]string) error {
	// routes
	dst, err := netip.ParsePrefix(ServerIP + "/32")
	nextHopVPNGateway, _ := netip.ParseAddr(config.LocalInterface.Gateway)
	err = localInterface.AddRoute(dst, nextHopVPNGateway, 1)
	if err != nil {
		return routingError(dst)
	}
	
	// Windows 排除路由 metric 相对大小好像不起作用，但不影响效果
	if len(*SplitInclude) == 0 {
		*SplitInclude = append(*SplitInclude, "0.0.0.0/0.0.0.0")
	}
	for _, ipMask := range *SplitInclude {
		dst, _ = netip.ParsePrefix(IpMaskToCIDR(ipMask))
		err = iface.AddRoute(dst, nextHopVPN, 6)
		if err != nil {
			return routingError(dst)
		}
	}
	
	if len(*SplitExclude) > 0 {
		for _, ipMask := range *SplitExclude {
			dst, _ = netip.ParsePrefix(IpMaskToCIDR(ipMask))
			err = localInterface.AddRoute(dst, nextHopVPNGateway, 5)
			if err != nil {
				return routingError(dst)
			}
		}
	}
	
	return err
}

func ResetRoutes(ServerIP string, DNS, SplitExclude []string) {
	dst, _ := netip.ParsePrefix(ServerIP + "/32")
	nextHopVPNGateway, _ := netip.ParseAddr(config.LocalInterface.Gateway)
	localInterface.DeleteRoute(dst, nextHopVPNGateway)
	
	if len(SplitExclude) > 0 {
		for _, ipMask := range SplitExclude {
			dst, _ = netip.ParsePrefix(IpMaskToCIDR(ipMask))
			localInterface.DeleteRoute(dst, nextHopVPNGateway)
		}
	}
}

func routingError(dst netip.Prefix) error {
	return fmt.Errorf("routing error: %s", dst.String())
}
