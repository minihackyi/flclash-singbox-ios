import 'dart:async';

import 'package:fl_clash/common/common.dart';
import 'package:fl_clash/state.dart';
import 'package:flutter/material.dart';

/// 轮询官网站内消息：约 5 秒一次，收到未读消息立即弹窗并带到前台。
class MessagePoller with WidgetsBindingObserver {
  MessagePoller._();

  static final MessagePoller instance = MessagePoller._();

  Timer? _timer;
  bool _observing = false;
  bool _checking = false;

  void start() {
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 5), (_) {
      check();
    });
    if (!_observing) {
      _observing = true;
      WidgetsBinding.instance.addObserver(this);
    }
    check();
  }

  void stop() {
    _timer?.cancel();
    _timer = null;
    if (_observing) {
      _observing = false;
      WidgetsBinding.instance.removeObserver(this);
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      check();
    }
  }

  Future<void> check() async {
    if (_checking || !authSession.isAuthenticated) return;
    _checking = true;
    try {
      final messages = await authSession.fetchMessages();
      if (messages.isEmpty) return;
      final ids = <int>[];
      var shown = false;
      for (final message in messages) {
        final id = message['id'];
        final context = globalState.navigatorKey.currentContext;
        if (context == null) continue;
        if (!shown) {
          shown = true;
          // 若窗口在托盘/后台，先把窗口带到前台再弹窗
          window?.show();
        }
        await globalState.showMessage(
          title: message['title']?.toString() ?? '系统消息',
          message: TextSpan(text: message['content']?.toString() ?? ''),
        );
        if (id is int) ids.add(id);
      }
      if (ids.isNotEmpty) {
        await authSession.markMessagesRead(ids);
      }
    } finally {
      _checking = false;
    }
  }
}
