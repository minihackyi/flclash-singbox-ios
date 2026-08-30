#import <Foundation/Foundation.h>
#import "PacketTunnel-Swift.h"

// C symbols referenced by the Go static library (see platform_ios.go).
// They forward to the Swift PacketTunnelBridge class.

int iosOpenTunBridge(const char* settingsJson) {
    NSString *json = [NSString stringWithUTF8String:settingsJson ? settingsJson : ""];
    return (int)[PacketTunnelBridge openTunWithSettingsJson:json];
}

int iosBindInterfaceBridge(int fd) {
    return (int)[PacketTunnelBridge bindInterfaceWithFd:fd];
}
