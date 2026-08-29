import NetworkExtension
import Foundation

// Bridges sing-box (Go, statically linked) to the Packet Tunnel Provider.
// Go exposes: coreStartServer() -> port, coreStopServer(),
// coreServerPort(); and calls back into Swift via iosOpenTunBridge /
// iosBindInterfaceBridge (declared in clash.h, implemented here).

let appGroupId = "group.com.follow.clash.flClash"

enum TunBridgeError: Error {
    case invalidSettings
    case noFlow
}

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

    var ipv4: [String] = obj["inet4"] as? [String] ?? []
    var ipv6: [String] = obj["inet6"] as? [String] ?? []
    if ipv4.isEmpty && ipv6.isEmpty {
        ipv4 = ["198.18.0.1/30"]
    }

    let ipv4Settings = NEIPv4Settings(addresses: [], subnetMasks: [])
    var addresses: [String] = []
    var masks: [String] = []
    for entry in ipv4 {
        let parts = entry.split(separator: "/")
        guard parts.count == 2, let addr = parts.first, let bits = Int(parts[1]) else { continue }
        addresses.append(String(addr))
        masks.append(PacketTunnelProvider.subnetMask(bits: bits))
    }
    ipv4Settings.addresses = addresses
    ipv4Settings.subnetMasks = masks
    settings.ipv4Settings = ipv4Settings

    if !ipv6.isEmpty {
        let ipv6Settings = NEIPv6Settings(addresses: [], networkPrefixLengths: [])
        var v6addresses: [String] = []
        var v6lengths: [NSNumber] = []
        for entry in ipv6 {
            let parts = entry.split(separator: "/")
            guard parts.count == 2, let addr = parts.first, let bits = Int(parts[1]) else { continue }
            v6addresses.append(String(addr))
            v6lengths.append(NSNumber(value: bits))
        }
        ipv6Settings.addresses = v6addresses
        ipv6Settings.networkPrefixLengths = v6lengths
        settings.ipv6Settings = ipv6Settings
    }

    if let routes = obj["route"] as? [String], !routes.isEmpty {
        let v4 = routes.filter { !$0.contains(":") }.compactMap { prefix -> NEIPv4Route? in
            let parts = prefix.split(separator: "/")
            guard parts.count == 2, let addr = parts.first, let bits = Int(parts[1]) else { return nil }
            return NEIPv4Route(destinationAddress: String(addr), destinationSubnetMask: PacketTunnelProvider.subnetMask(bits: bits))
        }
        let v6 = routes.filter { $0.contains(":") }.compactMap { prefix -> NEIPv6Route? in
            let parts = prefix.split(separator: "/")
            guard parts.count == 2, let addr = parts.first, let bits = Int(parts[1]) else { return nil }
            return NEIPv6Route(destinationAddress: String(addr), destinationNetworkPrefixLength: NSNumber(value: bits))
        }
        ipv4Settings.includedRoutes = v4.isEmpty ? [] : v4
        ipv6Settings.includedRoutes = v6.isEmpty ? [] : v6
        settings.ipv4Settings = ipv4Settings
        settings.ipv6Settings = settings.ipv6Settings
    }

    let sema = DispatchSemaphore(value: 0)
    var applyError: Error?
    PacketTunnelProvider.shared?.packetFlow.setTunnelNetworkSettings(settings) { error in
        applyError = error
        sema.signal()
    }
    let waitResult = sema.wait(timeout: .now() + 5)
    if waitResult == .timedOut || applyError != nil {
        return 0
    }
    guard let flow = PacketTunnelProvider.shared?.packetFlow else { return 0 }
    return flow.fileDescriptor
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

    override func handleAppMessage(_ messageData: Data, completionHandler: @escaping (Data?) -> Void) {
        // App pings the extension for the current core port.
        if let text = String(data: messageData, encoding: .utf8), text == "port" {
            let port = coreServerPort()
            completionHandler(String(port).data(using: .utf8))
            return
        }
        completionHandler(nil)
    }
}
