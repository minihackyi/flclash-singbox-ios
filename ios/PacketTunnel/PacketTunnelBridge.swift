import NetworkExtension
import Foundation

// Swift side of the Go <-> Swift tunnel bridge. Called from iosbridge.m,
// which provides the C symbols the Go static library references.

@objc class PacketTunnelBridge: NSObject {
    @objc static func openTun(withSettingsJson json: String) -> Int32 {
        guard let data = json.data(using: .utf8),
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

        // NEPacketTunnelFlow does not expose its utun fd publicly; the KVC
        // path below is the established way to hand the fd to a Go stack.
        let flow = provider.packetFlow
        return flow.value(forKey: "_flow") as? Int32 ?? 0
    }

    @objc static func bindInterface(withFd fd: Int32) -> Int32 {
        // Binding to the physical interface is optional on iOS: extension
        // traffic already bypasses the VPN's own routes.
        return 0
    }
}
