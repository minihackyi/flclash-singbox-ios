//go:build ios

package main

// iOS platform glue: implements adapter.PlatformInterface so sing-box's tun
// inbound is backed by the NetworkExtension's NEPacketTunnelFlow. The Swift
// side provides iosOpenTunBridge (applies NEPacketTunnelNetworkSettings and
// returns the flow's tun fd) and iosBindInterfaceBound (binds sockets to the
// default interface) as externals resolved at link time.

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"syscall"
	"unsafe"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"
	"golang.org/x/sys/unix"
)

/*
#include <stdlib.h>
int iosOpenTunBridge(const char* settingsJson);
int iosBindInterfaceBridge(int fd);
*/
import "C"

type iosPlatform struct {
	networkManager adapter.NetworkManager
	tunName        string
}

func newIOSPlatform() *iosPlatform {
	return &iosPlatform{}
}

var _ adapter.PlatformInterface = (*iosPlatform)(nil)

func (p *iosPlatform) Initialize(networkManager adapter.NetworkManager) error {
	p.networkManager = networkManager
	return nil
}

func (p *iosPlatform) UsePlatformAutoDetectInterfaceControl() bool {
	return true
}

func (p *iosPlatform) AutoDetectInterfaceControl(fd int) error {
	if res := int(C.iosBindInterfaceBridge(C.int(fd))); res != 0 {
		return errors.New("bind interface failed")
	}
	return nil
}

func (p *iosPlatform) UsePlatformInterface() bool {
	return true
}

type iosTunSettings struct {
	Inet4Address   []string `json:"inet4"`
	Inet6Address   []string `json:"inet6"`
	MTU            int      `json:"mtu"`
	Inet4RouteAddr []string `json:"route"`
	DNSServer      string   `json:"dns"`
}

func (p *iosPlatform) OpenInterface(options *tun.Options, platformOptions option.TunPlatformOptions) (tun.Tun, error) {
	if len(options.IncludeUID) > 0 || len(options.ExcludeUID) > 0 {
		return nil, errors.New("platform: unsupported uid options")
	}
	routeRanges, err := options.BuildAutoRouteRanges(true)
	if err != nil {
		return nil, err
	}
	settings := iosTunSettings{
		Inet4Address:   prefixesToStrings(options.Inet4Address),
		Inet6Address:   prefixesToStrings(options.Inet6Address),
		MTU:            int(options.MTU),
		Inet4RouteAddr: prefixesToStrings(routeRanges),
		DNSServer:      iosTunDNSServer(options),
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	cJSON := C.CString(string(data))
	defer C.free(unsafe.Pointer(cJSON))
	tunFd := int(C.iosOpenTunBridge(cJSON))
	if tunFd <= 0 {
		return nil, errors.New("open tun via swift bridge failed")
	}
	name, err := getTunnelName(tunFd)
	if err != nil {
		return nil, errors.Join(errors.New("query tun name"), err)
	}
	options.Name = name
	options.InterfaceMonitor.RegisterMyInterface(options.Name)
	dupFd, err := dupFD(tunFd)
	if err != nil {
		return nil, errors.Join(errors.New("dup tun file descriptor"), err)
	}
	options.FileDescriptor = dupFd
	p.tunName = name
	return tun.New(*options)
}

func prefixesToStrings(prefixes []netip.Prefix) []string {
	out := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		out = append(out, prefix.String())
	}
	return out
}

// iosTunDNSServer returns the second tun address (DNS hijack points at the
// tun gateway + 1).
func iosTunDNSServer(options *tun.Options) string {
	if len(options.Inet4Address) > 0 {
		return options.Inet4Address[0].Addr().Next().String()
	}
	if len(options.Inet6Address) > 0 {
		return options.Inet6Address[0].Addr().Next().String()
	}
	return "198.18.0.2"
}

func getTunnelName(fd int) (string, error) {
	return unix.GetsockoptString(
		fd,
		2, /* SYSPROTO_CONTROL */
		2, /* UTUN_OPT_IFNAME */
	)
}

func dupFD(fd int) (int, error) {
	return unix.FcntlInt(uintptr(fd), syscall.F_DUPFD_CLOEXEC, 0)
}

func (p *iosPlatform) UsePlatformDefaultInterfaceMonitor() bool {
	return false
}

func (p *iosPlatform) CreateDefaultInterfaceMonitor(logger.Logger) tun.DefaultInterfaceMonitor {
	return nil
}

func (p *iosPlatform) UsePlatformNetworkInterfaces() bool {
	return false
}

func (p *iosPlatform) NetworkInterfaces() ([]adapter.NetworkInterface, error) {
	return nil, nil
}

func (p *iosPlatform) UnderNetworkExtension() bool {
	return true
}

func (p *iosPlatform) NetworkExtensionIncludeAllNetworks() bool {
	return false
}

func (p *iosPlatform) ClearDNSCache() {}

func (p *iosPlatform) RequestPermissionForWIFIState() error {
	return nil
}

func (p *iosPlatform) ReadWIFIState() adapter.WIFIState {
	return adapter.WIFIState{}
}

func (p *iosPlatform) SystemCertificates() []string {
	return nil
}

func (p *iosPlatform) UsePlatformConnectionOwnerFinder() bool {
	return false
}

func (p *iosPlatform) FindConnectionOwner(*adapter.FindConnectionOwnerRequest) (*adapter.ConnectionOwner, error) {
	return nil, nil
}

func (p *iosPlatform) UsePlatformWIFIMonitor() bool {
	return false
}

func (p *iosPlatform) UsePlatformNotification() bool {
	return false
}

func (p *iosPlatform) SendNotification(*adapter.Notification) error {
	return nil
}

func (p *iosPlatform) MyInterfaceAddress() []netip.Addr {
	return nil
}

var iosPlatformInstance = newIOSPlatform()

// withIOSPlatform registers the platform interface the same way libbox's
// baseContext does, so the tun inbound routes through the NetworkExtension.
func withIOSPlatform(ctx context.Context) context.Context {
	dnsRegistry := include.DNSTransportRegistry()
	ctx = box.Context(ctx, include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry(), dnsRegistry, include.ServiceRegistry())
	return service.ContextWith[adapter.PlatformInterface](ctx, iosPlatformInstance)
}
