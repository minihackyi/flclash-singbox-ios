import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:fl_clash/common/common.dart';
import 'package:fl_clash/enum/enum.dart';
import 'package:fl_clash/models/core.dart';
import 'package:fl_clash/models/models.dart';

import 'interface.dart';

const _iosCoreChannel = MethodChannel('com.follow.clash/ios_core');

/// iOS core transport: the sing-box engine runs inside the Packet Tunnel
/// extension and serves the same framed Action protocol on a loopback TCP
/// port. Profile/geo files are mirrored into the app group container so the
/// extension sandbox can read them.
class CoreRemoteService extends CoreHandlerInterface {
  static CoreRemoteService? _instance;

  Socket? _socket;
  final Map<String, Completer> _callbackCompleterMap = {};
  Completer<bool> _connectedCompleter = Completer();
  String? _appGroupPath;
  Timer? _statusTimer;

  CoreRemoteService._internal();

  factory CoreRemoteService() {
    _instance ??= CoreRemoteService._internal();
    return _instance!;
  }

  Future<String> get _appGroupPath async {
    if (_appGroupPath != null) {
      return _appGroupPath!;
    }
    _appGroupPath = await _iosCoreChannel.invokeMethod<String>('appGroupPath');
    return _appGroupPath!;
  }

  /// Mirrors config + geo data into the app group container for the
  /// extension-side core.
  Future<String> get coreHomeDir async {
    final group = await _appGroupPath;
    final dir = Directory('$group/core_home');
    if (!dir.existsSync()) {
      dir.createSync(recursive: true);
    }
    final homeDirPath = await appPath.homeDirPath;
    for (final name in ['config.yaml', 'GEOIP.dat', 'GEOSITE.dat', 'GEOIP.metadb', 'ASN.mmdb']) {
      final src = File('$homeDirPath/$name');
      if (src.existsSync()) {
        src.copySync('${dir.path}/$name');
      }
    }
    return dir.path;
  }

  @override
  Future<bool> init(InitParams params) async {
    final coreHome = await coreHomeDir;
    final res = await super.init(
      InitParams(homeDir: coreHome, version: params.version),
    );
    await _iosCoreChannel.invokeMethod('startVpn');
    return res;
  }

  @override
  Future<String> preload() async {
    if (_connectedCompleter.isCompleted) {
      return 'core is connected';
    }
    // Wait for the extension to publish its port, then connect.
    for (var attempt = 0; attempt < 30; attempt++) {
      final port = await _iosCoreChannel.invokeMethod<int>('corePort') ?? 0;
      if (port > 0) {
        try {
          _socket = await Socket.connect('127.0.0.1', port);
          break;
        } catch (_) {
          await Future<void>.delayed(const Duration(milliseconds: 500));
        }
      } else {
        await Future<void>.delayed(const Duration(milliseconds: 500));
      }
    }
    final socket = _socket;
    if (socket == null) {
      return 'connect core failed';
    }
    _connectedCompleter.complete(true);
    socket.listen(
      _onData,
      onError: (_) => _handleDisconnect(),
      onDone: _handleDisconnect,
    );
    return '';
  }

  void _handleDisconnect() {
    _socket?.destroy();
    _socket = null;
    if (_connectedCompleter.isCompleted) {
      _connectedCompleter = Completer();
    }
  }

  void _onData(Uint8List chunk) {
    _buffer.addAll(chunk);
    while (true) {
      if (_buffer.length < 4) {
        return;
      }
      final length = _buffer[0] |
          (_buffer[1] << 8) |
          (_buffer[2] << 16) |
          (_buffer[3] << 24);
      if (_buffer.length < 4 + length) {
        return;
      }
      final data = utf8.decode(_buffer.sublist(4, 4 + length));
      _buffer.removeRange(0, 4 + length);
      try {
        final json = data.trim().commonToJSON<dynamic>();
        handleResult(ActionResult.fromJson(json));
      } catch (e) {
        commonPrint.log('parse core data failed: $e', logLevel: LogLevel.error);
      }
    }
  }

  final List<int> _buffer = [];

  Future<void> handleResult(ActionResult result) async {
    final completer = _callbackCompleterMap[result.id];
    final data = await parasResult(result);
    if (result.id.isEmpty) {
      coreEventManager.sendEvent(CoreEvent.fromJson(result.data));
      return;
    }
    if (completer?.isCompleted == true) {
      return;
    }
    completer?.complete(data);
  }

  @override
  Future<T?> invoke<T>({
    required ActionMethod method,
    dynamic data,
    Duration? timeout,
  }) async {
    try {
      await _connectedCompleter.future.timeout(const Duration(seconds: 20));
    } catch (_) {
      return null;
    }
    final id = '${method.name}#${utils.id}';
    _callbackCompleterMap[id] = Completer<T?>();
    final action = json.encode(Action(id: id, method: method, data: data));
    final payload = utf8.encode(action);
    final frame = BytesBuilder();
    frame.add([
      payload.length & 0xff,
      (payload.length >> 8) & 0xff,
      (payload.length >> 16) & 0xff,
      (payload.length >> 24) & 0xff,
    ]);
    frame.add(payload);
    _socket?.add(frame.takeBytes());
    try {
      return await (_callbackCompleterMap[id] as Completer<T?>).future.timeout(
            timeout ?? const Duration(seconds: 30),
            onTimeout: () => null,
          );
    } finally {
      _callbackCompleterMap.remove(id);
    }
  }

  @override
  Completer get completer => _connectedCompleter;

  @override
  FutureOr<bool> destroy() async {
    await shutdown(false);
    return true;
  }

  @override
  Future<bool> shutdown(bool isUser) async {
    _handleDisconnect();
    if (isUser) {
      await _iosCoreChannel.invokeMethod('stopVpn');
    }
    return true;
  }

  @override
  Future<bool> startListener() async {
    await super.startListener();
    _statusTimer?.cancel();
    _statusTimer = Timer.periodic(const Duration(seconds: 5), (_) async {
      final status = await _iosCoreChannel.invokeMethod<String>('vpnStatus');
      if (status != 'connected' && _connectedCompleter.isCompleted) {
        _handleDisconnect();
      }
    });
    return true;
  }

  @override
  Future<bool> stopListener() async {
    _statusTimer?.cancel();
    await super.stopListener();
    return true;
  }
}

CoreRemoteService? get coreRemote => system.isIOS ? CoreRemoteService() : null;
