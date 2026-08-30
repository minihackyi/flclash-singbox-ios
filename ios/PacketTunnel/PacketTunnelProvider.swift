import NetworkExtension
import Foundation

// Bridges sing-box (Go, statically linked) to the Packet Tunnel Provider.
// Go exposes: coreStartServer() -> port, coreStopServer(),
// coreServerPort(); and calls back into Swift via iosOpenTunBridge /
// iosBindInterfaceBridge (declared in the bridging header, implemented here).

let appGroupId = "group.com.follow.clash.flClash"

func iosOpenTunBridge(_ settingsJson: UnsafePointer<CChar>?) -> Int32 {
    guard let json = settingsJson else { return 0 }
    guard let data = String(cString: json).data(using: .utf8),
          let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
        return 0
    }

    let settings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "198.18.0.1")

    var mtu: Int = 9000
    if let m = obj["mtu"] as? Int, m > 0 { mtu = m }
    settings.mtu = NSNumber(value: mtu)

    if let dns = obj["dns"] as? String {
        settings.dnsSettings = NEDNSSettings(servers: [dns])
    }

    var ipv4Entries: [String] = obj["inet4"] as? [String] ?? []
    var ipv6Entries: [String] = obj["inet6"] as? [String] ?? []
    if ipv4Entries.isEmpty && ipv6Entries.isEmpty {
        ipv4Entries = ["198.18.0.1/30"]
    }

    var v4Addresses: [String] = []
    var v4Masks: [String] = []
    for entry in ipv4Entries {
        let parts = entry.split(separator: "/")
        guard parts.count == 2, let addr = parts.first, let bits = Int(parts[1]) else { continue }
        v4Addresses.append(String(addr))
        v4Masks.append(PacketTunnelProvider.subnetMask(bits: bits))
    }

    var v6Addresses: [String] = []
    var v6Lengths: [NSNumber] = []
    for entry in ipv6Entries {
        let parts = entry.split(separator: "/")
        guard parts.count == 2, let addr = parts.first, let bits = Int(parts[1]) else { continue }
        v6Addresses.append(String(addr))
        v6Lengths.append(NSNumber(value: bits))
    }

    var v4Routes: [NEIPv4Route] = []
    var v6Routes: [NEIPv6Route] = []
    if let routes = obj["route"] as? [String] {
        for entry in routes {
            let parts = entry.split(separator: "/")
            guard parts.count == 2, let addr = parts.first, let bits = Int(parts[1]) else { continue }
            if entry.contains(":") {
                v6Routes.append(NEIPv6Route(
                    destinationAddress: String(addr),
                    networkPrefixLength: NSNumber(value: bits)
                ))
            } else {
                v4Routes.append(NEIPv4Route(
                    destinationAddress: String(addr),
                    subnetMask: PacketTunnelProvider.subnetMask(bits: bits)
                ))
            }
        }
    }

    if !v4Addresses.isEmpty {
        let ipv4Settings = NEIPv4Settings(addresses: v4Addresses, subnetMasks: v4Masks)
        ipv4Settings.includedRoutes = v4Routes
        settings.ipv4Settings = ipv4Settings
    }
    if !v6Addresses.isEmpty {
        let ipv6Settings = NEIPv6Settings(addresses: v6Addresses, networkPrefixLengths: v6Lengths)
        ipv6Settings.includedRoutes = v6Routes
        settings.ipv6Settings = ipv6Settings
    }

    guard let provider = PacketTunnelProvider.shared else { return 0 }
    let sema = DispatchSemaphore(value: 0)
    var applyError: Error?
    provider.setTunnelNetworkSettings(settings) { error in
        applyError = error
        sema.signal()
    }
    if sema.wait(timeout: .now() + 5) == .timedOut || applyError != nil {
        return 0
    }

    // NEPacketTunnelFlow does not expose its utun fd publicly; the KVC path
    // below is the established way to hand the fd to a Go/gvisor stack.
    let flow = provider.packetFlow
    let fd = flow.value(forKey: "_flow") as? Int32 ?? 0
    return fd
}

func iosBindInterfaceBridge(_ fd: Int32) -> Int32 {
    // Binding to the physical interface is optional on iOS: extension
    // traffic already bypasses the VPN's own routes.
    return 0
}

class PacketTunnelProvider: NEPacketTunnelProvider {
    static var shared: PacketTunnelProvider?

    override init() {
        super.init()
        PacketTunnelProvider.shared = self
    }

    deinit {
        PacketTunnelProvider.shared = nil
    }

    static func subnetMask(bits: Int) -> String {
        var octets: [String] = []
        for octet in 0..<4 {
            let shift = max(0, min(8, bits - octet * 8))
            let value = shift == 0 ? 0 : (UInt8.max << (8 - UInt8(shift)))
            octets.append(String(value))
        }
        return octets.joined(separator: ".")
    }

    private func persistPort() {
        let port = coreStartServer()
        let defaults = UserDefaults(suiteName: appGroupId)
        defaults?.set(Int(port), forKey: "coreServerPort")
    }

    override func startTunnel(options: [String: NSObject]?) async throws {
        persistPort()
        // The Go engine creates the TUN asynchronously through
        // iosOpenTunBridge when the profile's tun inbound starts.
    }

    override func stopTunnel(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        coreStopServer()
        completionHandler()
    }

    override func handleAppMessage(_ messageData: Data, completionHandler: ((Data?) -> Void)?) {
        // App pings the extension for the current core port.
        guard let completionHandler else { return }
        if let text = String(data: messageData, encoding: .utf8), text == "port" {
            let port = coreServerPort()
            completionHandler(String(port).data(using: .utf8))
            return
        }
        completionHandler(nil)
    }
}
