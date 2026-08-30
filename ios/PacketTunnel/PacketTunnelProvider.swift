import NetworkExtension
import Foundation

// Packet Tunnel Provider: hosts the Go sing-box core (statically linked)
// and exposes the loopback action server + tun bridge callbacks.

let appGroupId = "group.com.follow.clash.flClash"

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
        // PacketTunnelBridge when the profile's tun inbound starts.
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
