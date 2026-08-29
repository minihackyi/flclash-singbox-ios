import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'constant.dart';

class AdminVerifyResult {
  const AdminVerifyResult({required this.success, this.error});

  final bool success;
  final String? error;
}

class TrafficReportResult {
  const TrafficReportResult({
    this.success = false,
    this.status = 'active',
    this.message,
  });

  final bool success;
  final String status;
  final String? message;
}

class AuthSession extends ChangeNotifier {
  static const _tokenKey = 'xiaohei_auth_token';
  static const _userKey = 'xiaohei_auth_user';
  static const _channelKey = 'xiaohei_auth_channel';
  static const _serverKey = 'xiaohei_auth_server';
  static const _lastSubLinkKey = 'xiaohei_last_sub_link';

  String? _token;
  Map<String, dynamic>? _user;
  String? _lastSubLink;
  String _baseUrl = xiaoheiApiBase;
  String? _customServer;

  String? get token => _token;
  Map<String, dynamic>? get user => _user;
  bool get isAuthenticated => _token != null && _token!.isNotEmpty;
  String get baseUrl => _customServer ?? _baseUrl;
  String? get subLink => _user?['subLink']?.toString();
  String? get lastSubLink => _lastSubLink;
  String get subscriptionStatus => _user?['status']?.toString() ?? 'active';
  bool get subscriptionRevoked =>
      _user?['revokeReason'] != null ||
      subscriptionStatus == 'expired' ||
      subscriptionStatus == 'exhausted' ||
      subscriptionStatus == 'blacklisted';
  bool get usingBackupChannel =>
      _customServer == null && _baseUrl == xiaoheiBackupApiBase;
  bool get hasCustomServer =>
      _customServer != null && _customServer!.isNotEmpty;

  Future<void> restore() async {
    final prefs = await SharedPreferences.getInstance();
    final customServer = prefs.getString(_serverKey);
    if (customServer != null && customServer.isNotEmpty) {
      _customServer = customServer;
    } else if (prefs.getBool(_channelKey) == true) {
      _baseUrl = xiaoheiBackupApiBase;
    }
    _token = prefs.getString(_tokenKey);
    _lastSubLink = prefs.getString(_lastSubLinkKey);
    final rawUser = prefs.getString(_userKey);
    if (rawUser != null && rawUser.isNotEmpty) {
      try {
        _user = Map<String, dynamic>.from(jsonDecode(rawUser) as Map);
      } catch (_) {
        _user = null;
      }
    }
    notifyListeners();
  }

  Future<String?> login(String username, String password) {
    return _authWithFallback(username, password);
  }

  /// 注册官网账号，成功后自动登录（返回 null 表示成功）。
  Future<String?> register(
    String username,
    String password,
    String phone,
  ) {
    return _authWithFallback(username, password, phone: phone);
  }

  Future<String?> _authWithFallback(
    String username,
    String password, {
    String? phone,
  }) async {
    for (final base in _requestBases()) {
      final error = await _loginTo(base, username, password, phone: phone);
      if (error == null) {
        if (base != _baseUrl && !hasCustomServer) {
          _baseUrl = base;
          final prefs = await SharedPreferences.getInstance();
          await prefs.setBool(_channelKey, base == xiaoheiBackupApiBase);
        }
        return null;
      }
      if (error != '无法连接官网，请检查网络或切换通道') return error;
    }
    return '无法连接官网，请检查网络或切换通道';
  }

  Future<String?> _loginTo(
    String base,
    String username,
    String password, {
    String? phone,
  }) async {
    try {
      final response =
          await Dio(
            BaseOptions(
              connectTimeout: const Duration(seconds: 10),
              receiveTimeout: const Duration(seconds: 15),
              validateStatus: (status) => status != null && status < 500,
            ),
          ).post<Map<String, dynamic>>(
            phone == null ? '$base/api/login' : '$base/api/register',
            data: phone == null
                ? {'username': username, 'password': password}
                : {
                    'username': username,
                    'password': password,
                    'phone': phone,
                  },
          );
      final data = response.data ?? <String, dynamic>{};
      if (response.statusCode != 200 ||
          data['success'] == false ||
          data['token'] == null) {
        return data['error']?.toString() ?? '注册或登录失败';
      }
      _token = data['token']?.toString();
      _user = data['user'] is Map
          ? Map<String, dynamic>.from(data['user'] as Map)
          : null;
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_tokenKey, _token!);
      if (_user != null) {
        await prefs.setString(_userKey, jsonEncode(_user));
        _rememberSubLink(_user);
      }
      notifyListeners();
      return null;
    } on DioException catch (error) {
      final responseData = error.response?.data;
      if (responseData is Map) {
        return responseData['error']?.toString() ?? '无法连接官网';
      }
      return '无法连接官网，请检查网络或切换通道';
    } catch (_) {
      return '无法连接官网，请检查网络或切换通道';
    }
  }

  Future<Map<String, dynamic>?> _requestUser(String baseUrl) async {
    if (_token == null || _token!.isEmpty) return null;
    try {
      final response =
          await Dio(
            BaseOptions(
              connectTimeout: const Duration(seconds: 10),
              receiveTimeout: const Duration(seconds: 15),
              validateStatus: (status) => status != null && status < 500,
            ),
          ).get<Map<String, dynamic>>(
            '$baseUrl/api/user/info',
            options: Options(headers: {'Authorization': 'Bearer $_token'}),
          );
      final data = response.data;
      if (response.statusCode != 200 || data == null) return null;
      if (data['user'] is Map) {
        return Map<String, dynamic>.from(data['user'] as Map);
      }
      if (data['username'] != null) {
        return Map<String, dynamic>.from(data);
      }
      return null;
    } catch (_) {
      return null;
    }
  }

  Future<void> _saveUser() async {
    final prefs = await SharedPreferences.getInstance();
    if (_user != null) {
      await prefs.setString(_userKey, jsonEncode(_user));
    }
  }

  void _rememberSubLink(Map<String, dynamic>? user) {
    final link = user?['subLink']?.toString();
    if (link != null && link.startsWith('http')) {
      _lastSubLink = link;
      SharedPreferences.getInstance().then(
        (prefs) => prefs.setString(_lastSubLinkKey, link),
      );
    }
  }

  /// 上报本机累计流量增量（字节），服务器按 GB 累计到账号流量。
  Future<TrafficReportResult> reportTraffic({
    required num up,
    required num down,
  }) {
    return _postUserStatus('/api/user/traffic', {'up': up, 'down': down});
  }

  /// 在线心跳：代理运行时定时上报，服务器用于判断用户是否在线。
  Future<TrafficReportResult> heartbeat({bool online = true}) {
    return _postUserStatus('/api/user/heartbeat', {'online': online});
  }

  Future<TrafficReportResult> _postUserStatus(
    String path,
    Map<String, dynamic> data,
  ) async {
    if (!isAuthenticated) return const TrafficReportResult();
    for (final base in _requestBases()) {
      try {
        final response = await Dio(
          BaseOptions(
            connectTimeout: const Duration(seconds: 10),
            receiveTimeout: const Duration(seconds: 15),
            validateStatus: (status) => status != null && status < 500,
          ),
        ).post<Map<String, dynamic>>(
          '$base$path',
          data: data,
          options: Options(headers: {'Authorization': 'Bearer $_token'}),
        );
        final responseData = response.data;
        if (response.statusCode != 200 ||
            responseData == null ||
            responseData['success'] == false) {
          if (response.statusCode != null &&
              response.statusCode! >= 400 &&
              response.statusCode! < 500) {
            return const TrafficReportResult();
          }
          continue;
        }
        final user = responseData['user'];
        if (user is Map) {
          _user = Map<String, dynamic>.from(user);
          await _saveUser();
          _rememberSubLink(_user);
          notifyListeners();
        }
        return TrafficReportResult(
          success: true,
          status: responseData['status']?.toString() ?? 'active',
          message: responseData['message']?.toString(),
        );
      } catch (_) {
        // 网络失败则尝试备用通道
      }
    }
    return const TrafficReportResult();
  }

  /// 按顺序返回可用的服务端地址（自定义服务器优先，其次主/备用官网）。
  List<String> _requestBases() {
    if (hasCustomServer) return [baseUrl];
    if (_baseUrl == xiaoheiBackupApiBase) {
      return [xiaoheiBackupApiBase, xiaoheiApiBase];
    }
    return [xiaoheiApiBase, xiaoheiBackupApiBase];
  }

  /// 校验管理员账号密码（与网页 admin 后台一致），用于客户端受保护操作。
  /// 复用已有的 /api/admin/login + /api/admin/logout，兼容未重启的旧服务端。
  Future<AdminVerifyResult> verifyAdmin(String username, String password) async {
    if (username.isEmpty || password.isEmpty) {
      return const AdminVerifyResult(
        success: false,
        error: '请输入管理员账号和密码',
      );
    }
    for (final base in _requestBases()) {
      try {
        final loginResponse = await Dio(
          BaseOptions(
            connectTimeout: const Duration(seconds: 10),
            receiveTimeout: const Duration(seconds: 15),
            validateStatus: (status) => status != null && status < 500,
          ),
        ).post<Map<String, dynamic>>(
          '$base/api/admin/login',
          data: {'username': username, 'password': password},
        );
        final loginData = loginResponse.data ?? <String, dynamic>{};
        if (loginResponse.statusCode == 200 &&
            loginData['success'] == true &&
            loginData['token'] != null) {
          final adminToken = loginData['token']?.toString();
          if (adminToken != null && adminToken.isNotEmpty) {
            try {
              await Dio(
                BaseOptions(
                  validateStatus: (status) => status != null && status < 500,
                ),
              ).post<Map<String, dynamic>>(
                '$base/api/admin/logout',
                options: Options(
                  headers: {'Authorization': 'Bearer $adminToken'},
                ),
              );
            } catch (_) {
              // 登出失败不影响验证结果
            }
          }
          return const AdminVerifyResult(success: true);
        }
        if (loginResponse.statusCode == 401) {
          return const AdminVerifyResult(
            success: false,
            error: '管理员账号或密码错误',
          );
        }
      } catch (_) {
        // 网络失败则尝试备用通道
      }
    }
    return const AdminVerifyResult(
      success: false,
      error: '无法连接服务器验证，请检查网络或确认服务端已重启',
    );
  }

  /// 获取未读的站内弹窗消息。
  Future<List<Map<String, dynamic>>> fetchMessages() async {
    if (!isAuthenticated) return [];
    for (final base in _requestBases()) {
      try {
        final response = await Dio(
          BaseOptions(
            connectTimeout: const Duration(seconds: 10),
            receiveTimeout: const Duration(seconds: 15),
            validateStatus: (status) => status != null && status < 500,
          ),
        ).get<Map<String, dynamic>>(
          '$base/api/user/messages',
          options: Options(headers: {'Authorization': 'Bearer $_token'}),
        );
        final data = response.data;
        if (response.statusCode == 200 &&
            data != null &&
            data['messages'] is List) {
          return List<Map<String, dynamic>>.from(
            (data['messages'] as List).map(
              (e) => Map<String, dynamic>.from(e as Map),
            ),
          );
        }
      } catch (_) {
        // 网络失败则尝试备用通道
      }
    }
    return [];
  }

  /// 标记站内消息为已读。
  Future<void> markMessagesRead(List<int> ids) async {
    if (!isAuthenticated || ids.isEmpty) return;
    for (final base in _requestBases()) {
      try {
        await Dio(
          BaseOptions(
            connectTimeout: const Duration(seconds: 10),
            receiveTimeout: const Duration(seconds: 15),
            validateStatus: (status) => status != null && status < 500,
          ),
        ).post<Map<String, dynamic>>(
          '$base/api/user/messages/read',
          data: {'ids': ids},
          options: Options(headers: {'Authorization': 'Bearer $_token'}),
        );
        return;
      } catch (_) {
        // 尝试备用通道
      }
    }
  }

  /// Refresh account data from the official website.
  ///
  /// If the selected channel cannot be reached, the backup channel is tried
  /// once. A successful backup response also updates the persisted channel so
  /// subsequent login and subscription requests remain usable.
  Future<String?> refreshUser() async {
    if (!isAuthenticated) return '请先登录官网账号';

    final user = await _requestUser(baseUrl);
    if (user != null) {
      _user = user;
      await _saveUser();
      _rememberSubLink(_user);
      notifyListeners();
      return null;
    }

    if (!hasCustomServer && _baseUrl != xiaoheiBackupApiBase) {
      final backupUser = await _requestUser(xiaoheiBackupApiBase);
      if (backupUser != null) {
        _baseUrl = xiaoheiBackupApiBase;
        _user = backupUser;
        final prefs = await SharedPreferences.getInstance();
        await prefs.setBool(_channelKey, true);
        await _saveUser();
        _rememberSubLink(_user);
        notifyListeners();
        return null;
      }
    }
    return '无法获取官网用户信息';
  }

  Future<void> logout() async {
    _token = null;
    _user = null;
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenKey);
    await prefs.remove(_userKey);
    notifyListeners();
  }

  Future<void> setChannel({required bool backup}) async {
    _baseUrl = backup ? xiaoheiBackupApiBase : xiaoheiApiBase;
    _customServer = null;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_channelKey, backup);
    await prefs.remove(_serverKey);
    notifyListeners();
  }

  /// Point the client at a self-hosted official server (for example a local
  /// development server such as http://127.0.0.1:8080). This overrides the
  /// main/backup channel until [clearCustomServer] or [setChannel] is called.
  Future<void> setCustomServer(String url) async {
    var normalized = url.trim();
    while (normalized.endsWith('/')) {
      normalized = normalized.substring(0, normalized.length - 1);
    }
    if (normalized.isEmpty ||
        (!normalized.startsWith('http://') &&
            !normalized.startsWith('https://'))) {
      throw ArgumentError('请输入以 http:// 或 https:// 开头的服务器地址');
    }
    _customServer = normalized;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_serverKey, normalized);
    await prefs.setBool(_channelKey, false);
    notifyListeners();
  }

  Future<void> clearCustomServer() async {
    _customServer = null;
    _baseUrl = xiaoheiApiBase;
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_serverKey);
    await prefs.setBool(_channelKey, false);
    notifyListeners();
  }
}

final authSession = AuthSession();
