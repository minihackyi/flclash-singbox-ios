import 'package:fl_clash/common/common.dart';
import 'package:fl_clash/common/message_poller.dart';
import 'package:fl_clash/providers/providers.dart';
import 'package:fl_clash/state.dart';
import 'dart:async';
import 'package:flutter/material.dart';

class AuthGate extends StatefulWidget {
  final Widget child;

  const AuthGate({super.key, required this.child});

  @override
  State<AuthGate> createState() => _AuthGateState();
}

class _AuthGateState extends State<AuthGate> {
  bool _restored = false;
  bool _startupSubscriptionChecked = false;

  @override
  void initState() {
    super.initState();
    authSession.addListener(_handleSessionChanged);
    unawaited(_restoreSession());
  }

  Future<void> _restoreSession() async {
    await authSession.restore();
    if (!mounted) return;
    setState(() => _restored = true);
    if (authSession.isAuthenticated) {
      await _checkSubscriptionOnStartup();
    }
  }

  Future<bool> _waitForAppReady() async {
    for (var i = 0; i < 50; i++) {
      if (!mounted) return false;
      if (globalState.container.read(initProvider)) return true;
      await Future.delayed(const Duration(milliseconds: 200));
    }
    return globalState.container.read(initProvider);
  }

  Future<void> _checkSubscriptionOnStartup() async {
    if (_startupSubscriptionChecked) return;
    _startupSubscriptionChecked = true;
    await _waitForAppReady();
    if (!mounted || !authSession.isAuthenticated) return;
    final result = await globalState.container
        .read(profilesActionProvider.notifier)
        .syncOfficialSubscription();
    if (!mounted) return;
    if (result.revoked) {
      globalState.showMessage(
        title: '订阅提醒',
        message: TextSpan(
          text: '${result.message ?? '订阅已失效'}\n订阅已自动删除，请联系管理员续期或补充流量。',
        ),
      );
    } else if (!result.success) {
      globalState.showNotifier('订阅检测失败：${result.message ?? '未知错误'}');
    } else if (result.changed) {
      globalState.showNotifier('官网订阅已更新，已自动导入');
    }
  }

  void _handleSessionChanged() {
    if (authSession.isAuthenticated) {
      MessagePoller.instance.start();
    } else {
      MessagePoller.instance.stop();
    }
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    authSession.removeListener(_handleSessionChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (!_restored) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    return authSession.isAuthenticated
        ? widget.child
        : LoginView(onLoggedIn: () => setState(() {}));
  }
}

class LoginView extends StatefulWidget {
  final VoidCallback onLoggedIn;

  const LoginView({super.key, required this.onLoggedIn});

  @override
  State<LoginView> createState() => _LoginViewState();
}

class _LoginViewState extends State<LoginView> {
  final _usernameController = TextEditingController();
  final _passwordController = TextEditingController();
  bool _loading = false;
  String? _error;

  @override
  void dispose() {
    _usernameController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  Future<void> _login() async {
    final username = _usernameController.text.trim();
    final password = _passwordController.text;
    if (username.isEmpty || password.isEmpty) {
      setState(() => _error = '请输入官网账号和密码');
      return;
    }
    setState(() {
      _loading = true;
      _error = null;
    });
    final error = await authSession.login(username, password);
    if (!mounted) return;
    if (error != null) {
      setState(() {
        _loading = false;
        _error = error;
      });
      return;
    }
    final result = await globalState.container
        .read(profilesActionProvider.notifier)
        .syncOfficialSubscription(refreshUser: false);
    if (!mounted) return;
    setState(() => _loading = false);
    widget.onLoggedIn();
    if (result.revoked) {
      globalState.showMessage(
        title: '订阅提醒',
        message: TextSpan(
          text: '${result.message ?? '订阅已失效'}\n订阅已自动删除，请联系管理员续期或补充流量。',
        ),
      );
    } else if (!result.success) {
      context.showNotifier('登录成功，${result.message ?? '订阅导入失败'}');
    } else if (result.created) {
      context.showNotifier('登录成功，订阅已自动导入');
    } else if (result.changed) {
      context.showNotifier('登录成功，订阅已更新');
    }
  }

  Future<void> _showRegisterDialog() async {
    final usernameController = TextEditingController();
    final phoneController = TextEditingController();
    final passwordController = TextEditingController();
    final confirmController = TextEditingController();
    String? formError;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) {
        return StatefulBuilder(
          builder: (dialogContext, setDialogState) {
            return AlertDialog(
              title: const Text('注册官网账号'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    TextField(
                      controller: usernameController,
                      textInputAction: TextInputAction.next,
                      decoration: const InputDecoration(
                        labelText: '用户名（至少3个字符）',
                        prefixIcon: Icon(Icons.person_outline),
                      ),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: phoneController,
                      keyboardType: TextInputType.phone,
                      textInputAction: TextInputAction.next,
                      decoration: const InputDecoration(
                        labelText: '手机号（11位，用于找回密码）',
                        prefixIcon: Icon(Icons.phone_outlined),
                      ),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: passwordController,
                      obscureText: true,
                      textInputAction: TextInputAction.next,
                      decoration: const InputDecoration(
                        labelText: '密码（至少4个字符）',
                        prefixIcon: Icon(Icons.lock_outline),
                      ),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: confirmController,
                      obscureText: true,
                      onSubmitted: (_) => Navigator.of(dialogContext).pop(true),
                      decoration: const InputDecoration(
                        labelText: '确认密码',
                        prefixIcon: Icon(Icons.lock_outline),
                      ),
                    ),
                    if (formError != null) ...[
                      const SizedBox(height: 10),
                      Text(
                        formError!,
                        style: TextStyle(
                          color: Theme.of(dialogContext).colorScheme.error,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.of(dialogContext).pop(false),
                  child: const Text('取消'),
                ),
                FilledButton(
                  onPressed: () {
                    final username = usernameController.text.trim();
                    final phone = phoneController.text.trim();
                    final password = passwordController.text;
                    final confirm = confirmController.text;
                    if (username.length < 3) {
                      setDialogState(() => formError = '用户名至少3个字符');
                      return;
                    }
                    if (!RegExp(r'^\d{11}$').hasMatch(phone)) {
                      setDialogState(() => formError = '请填写有效的11位手机号');
                      return;
                    }
                    if (password.length < 4) {
                      setDialogState(() => formError = '密码至少4个字符');
                      return;
                    }
                    if (password != confirm) {
                      setDialogState(() => formError = '两次输入的密码不一致');
                      return;
                    }
                    Navigator.of(dialogContext).pop(true);
                  },
                  child: const Text('注册'),
                ),
              ],
            );
          },
        );
      },
    );
    final username = usernameController.text.trim();
    final phone = phoneController.text.trim();
    final password = passwordController.text;
    usernameController.dispose();
    phoneController.dispose();
    passwordController.dispose();
    confirmController.dispose();
    if (confirmed != true || !mounted) return;
    if (username.isEmpty || phone.isEmpty || password.isEmpty) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    final registerError = await authSession.register(username, password, phone);
    if (!mounted) return;
    if (registerError != null) {
      setState(() {
        _loading = false;
        _error = '注册失败：$registerError';
      });
      return;
    }
    final result = await globalState.container
        .read(profilesActionProvider.notifier)
        .syncOfficialSubscription(refreshUser: false);
    if (!mounted) return;
    setState(() => _loading = false);
    widget.onLoggedIn();
    if (result.revoked) {
      globalState.showMessage(
        title: '订阅提醒',
        message: TextSpan(text: '${result.message ?? '订阅已失效'}\n请联系管理员开通套餐或续期。'),
      );
    } else if (!result.success) {
      context.showNotifier('注册成功，${result.message ?? '订阅导入失败'}');
    } else if (result.created) {
      context.showNotifier('注册成功，订阅已自动导入');
    } else if (result.changed) {
      context.showNotifier('注册成功，订阅已更新');
    }
  }

  Future<void> _showServerSettings() async {
    final result = await showDialog<String>(
      context: context,
      builder: (dialogContext) {
        final controller = TextEditingController(text: authSession.baseUrl);
        return AlertDialog(
          title: const Text('官网服务器地址'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                '填写小黑机场后端地址，例如 http://xiaohei.asia。保存后登录、用户信息和订阅检测都会使用该地址。',
                style: Theme.of(dialogContext).textTheme.bodySmall,
              ),
              const SizedBox(height: 12),
              TextField(
                controller: controller,
                autofocus: true,
                keyboardType: TextInputType.url,
                decoration: const InputDecoration(
                  hintText: 'http://xiaohei.asia',
                ),
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(dialogContext).pop('__clear__'),
              child: const Text('恢复默认'),
            ),
            FilledButton(
              onPressed: () => Navigator.of(dialogContext).pop(controller.text),
              child: const Text('保存'),
            ),
          ],
        );
      },
    );
    if (result == null || !mounted) return;
    if (result == '__clear__') {
      await authSession.clearCustomServer();
    } else {
      try {
        await authSession.setCustomServer(result);
      } on ArgumentError catch (e) {
        if (mounted) setState(() => _error = e.message.toString());
        return;
      }
    }
    if (mounted) setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final scheme = context.colorScheme;
    return Scaffold(
      backgroundColor: scheme.surface,
      body: Center(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Card(
              elevation: 0,
              color: scheme.surfaceContainer,
              child: Padding(
                padding: const EdgeInsets.all(28),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Icon(Icons.flight_takeoff, size: 54, color: scheme.primary),
                    const SizedBox(height: 16),
                    Text(
                      '登录小黑机场',
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.headlineSmall
                          ?.copyWith(fontWeight: FontWeight.w700),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      '使用官网账号进入客户端，订阅将自动导入',
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 28),
                    TextField(
                      controller: _usernameController,
                      textInputAction: TextInputAction.next,
                      decoration: const InputDecoration(
                        labelText: '官网账号',
                        prefixIcon: Icon(Icons.person_outline),
                      ),
                    ),
                    const SizedBox(height: 14),
                    TextField(
                      controller: _passwordController,
                      obscureText: true,
                      onSubmitted: (_) => _login(),
                      decoration: const InputDecoration(
                        labelText: '官网密码',
                        prefixIcon: Icon(Icons.lock_outline),
                      ),
                    ),
                    if (_error != null) ...[
                      const SizedBox(height: 12),
                      Text(_error!, style: TextStyle(color: scheme.error)),
                    ],
                    const SizedBox(height: 22),
                    FilledButton.icon(
                      onPressed: _loading ? null : _login,
                      icon: _loading
                          ? const SizedBox(
                              width: 18,
                              height: 18,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.login),
                      label: Text(_loading ? '正在登录…' : '登录并导入订阅'),
                    ),
                    const SizedBox(height: 8),
                    TextButton(
                      onPressed: _loading ? null : _showRegisterDialog,
                      child: const Text('没有官网账号？立即注册'),
                    ),
                    const SizedBox(height: 8),
                    TextButton.icon(
                      onPressed: _loading
                          ? null
                          : () async {
                              await authSession.setChannel(
                                backup: !authSession.usingBackupChannel,
                              );
                              if (mounted) setState(() {});
                            },
                      icon: const Icon(Icons.swap_horiz),
                      label: Text(
                        authSession.usingBackupChannel
                            ? '当前：备用官网通道'
                            : '当前：主官网通道（切换备用）',
                      ),
                    ),
                    const SizedBox(height: 4),
                    OutlinedButton.icon(
                      onPressed: _loading ? null : _showServerSettings,
                      icon: const Icon(Icons.dns_outlined),
                      label: const Text('设置服务器地址'),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      authSession.hasCustomServer
                          ? '官网：${authSession.baseUrl}（自定义）'
                          : '官网：${authSession.baseUrl}',
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
