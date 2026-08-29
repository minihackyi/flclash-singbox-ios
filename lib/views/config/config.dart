import 'package:fl_clash/common/common.dart';
import 'package:fl_clash/models/models.dart';
import 'package:fl_clash/providers/providers.dart';
import 'package:fl_clash/state.dart';
import 'package:fl_clash/widgets/widgets.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

class ConfigView extends ConsumerStatefulWidget {
  const ConfigView({super.key});

  @override
  ConsumerState<ConfigView> createState() => _ConfigViewState();
}

class _ConfigViewState extends ConsumerState<ConfigView> {
  bool _syncing = false;

  Profile? _findSubscriptionProfile(
    List<Profile> profiles,
    String? subscriptionUrl,
  ) {
    if (subscriptionUrl == null || subscriptionUrl.isEmpty) return null;
    for (final profile in profiles) {
      if (profile.url == subscriptionUrl) return profile;
    }
    return null;
  }

  DateTime? _parseExpireDate(String? value) {
    if (value == null || value.isEmpty) return null;
    if (value.startsWith('1000-') || value.startsWith('2000-01-01')) {
      return null;
    }
    return DateTime.tryParse(value.replaceAll(' ', 'T'));
  }

  String _formatDate(DateTime? value) {
    if (value == null) return '待开通';
    final month = value.month.toString().padLeft(2, '0');
    final day = value.day.toString().padLeft(2, '0');
    final hour = value.hour.toString().padLeft(2, '0');
    final minute = value.minute.toString().padLeft(2, '0');
    return '${value.year}-$month-$day $hour:$minute';
  }

  String _formatTraffic(dynamic value) {
    final gb = num.tryParse(value?.toString() ?? '');
    if (gb == null) return '--';
    if (gb >= 1024) return '${(gb / 1024).toStringAsFixed(2)} TB';
    return '${gb.toStringAsFixed(2)} GB';
  }

  String _formatDateTime(DateTime value) {
    final month = value.month.toString().padLeft(2, '0');
    final day = value.day.toString().padLeft(2, '0');
    final hour = value.hour.toString().padLeft(2, '0');
    final minute = value.minute.toString().padLeft(2, '0');
    return '${value.year}-$month-$day $hour:$minute';
  }

  String _subscriptionStatus(Profile? profile) {
    if (profile == null) return '未导入';
    return '已导入';
  }

  Future<void> _syncSubscription() async {
    if (_syncing) return;
    setState(() => _syncing = true);
    final result = await ref
        .read(profilesActionProvider.notifier)
        .syncOfficialSubscription();
    if (!mounted) return;
    setState(() => _syncing = false);
    if (result.revoked) {
      globalState.showMessage(
        title: '订阅提醒',
        message: TextSpan(
          text:
              '${result.message ?? '订阅已失效'}\n订阅已自动删除，请联系管理员续期或补充流量。',
        ),
      );
    } else if (!result.success) {
      context.showNotifier('同步失败：${result.message ?? '未知错误'}');
    } else if (result.changed || result.created) {
      context.showNotifier('订阅已更新并自动导入');
    } else {
      context.showNotifier('订阅已是最新');
    }
  }

  Future<void> _switchChannel() async {
    await authSession.setChannel(backup: !authSession.usingBackupChannel);
    if (!mounted) return;
    await _syncSubscription();
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
        if (mounted) context.showNotifier(e.message.toString());
        return;
      }
    }
    if (!mounted) return;
    await _syncSubscription();
  }

  Widget _infoRow(IconData icon, String label, String value) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Row(
        children: [
          Icon(icon, size: 20, color: scheme.onSurfaceVariant),
          const SizedBox(width: 14),
          Expanded(
            child: Text(
              label,
              style: Theme.of(
                context,
              ).textTheme.bodyMedium?.copyWith(color: scheme.onSurfaceVariant),
            ),
          ),
          const SizedBox(width: 16),
          Flexible(
            child: Text(
              value,
              textAlign: TextAlign.right,
              style: Theme.of(
                context,
              ).textTheme.bodyMedium?.copyWith(fontWeight: FontWeight.w600),
            ),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final profiles = ref.watch(profilesProvider);
    return ListenableBuilder(
      listenable: authSession,
      builder: (context, _) {
        final user = authSession.user ?? const <String, dynamic>{};
        final username = (user['username']?.toString() ?? '--').hideBrandName(fallback: '--');
        final plan = (user['plan']?.toString() ?? '未开通').hideBrandName(fallback: '未开通');
        final planDesc = user['planDesc']?.toString().hideBrandName();
        final expireRaw = user['expireDate']?.toString();
        final expireDate = _parseExpireDate(expireRaw);
        final subscriptionUrl = user['subLink']?.toString();
        final profile = _findSubscriptionProfile(profiles, subscriptionUrl);
        final now = DateTime.now();
        final daysLeft = expireDate?.difference(now).inDays;
        final expired = expireDate != null && expireDate.isBefore(now);
        final pending = plan == '待开通' || expireDate == null;
        final statusColor = pending
            ? scheme.tertiary
            : expired
            ? scheme.error
            : scheme.primary;

        return BaseScaffold(
          title: '用户信息',
          body: ListView(
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 28),
            children: [
              if (authSession.subscriptionStatus == 'expired' ||
                  authSession.subscriptionStatus == 'exhausted') ...[
                Card(
                  elevation: 0,
                  margin: EdgeInsets.zero,
                  color: scheme.errorContainer,
                  child: Padding(
                    padding: const EdgeInsets.all(14),
                    child: Row(
                      children: [
                        Icon(
                          Icons.warning_amber_rounded,
                          color: scheme.onErrorContainer,
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Text(
                            authSession.subscriptionStatus == 'expired'
                                ? '订阅已到期，请联系管理员续期'
                                : '流量已用完，请联系管理员补充流量',
                            style: Theme.of(context).textTheme.bodyMedium
                                ?.copyWith(
                                  color: scheme.onErrorContainer,
                                  fontWeight: FontWeight.w600,
                                ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: 14),
              ],
              Card(
                elevation: 0,
                margin: EdgeInsets.zero,
                color: scheme.surfaceContainer,
                child: Padding(
                  padding: const EdgeInsets.all(20),
                  child: Row(
                    children: [
                      CircleAvatar(
                        radius: 28,
                        backgroundColor: scheme.primaryContainer,
                        foregroundColor: scheme.onPrimaryContainer,
                        child: Text(
                          username.isEmpty
                              ? '?'
                              : String.fromCharCode(username.runes.first),
                          style: const TextStyle(
                            fontSize: 22,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ),
                      const SizedBox(width: 16),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              username,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: Theme.of(context).textTheme.titleMedium
                                  ?.copyWith(fontWeight: FontWeight.w700),
                            ),
                            const SizedBox(height: 6),
                            Text(
                              plan,
                              style: Theme.of(context).textTheme.bodyMedium
                                  ?.copyWith(color: scheme.onSurfaceVariant),
                            ),
                            if (planDesc != null && planDesc.isNotEmpty) ...[
                              const SizedBox(height: 2),
                              Text(
                                planDesc,
                                style: Theme.of(context).textTheme.bodySmall
                                    ?.copyWith(color: scheme.onSurfaceVariant),
                              ),
                            ],
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 14),
              Card(
                elevation: 0,
                margin: EdgeInsets.zero,
                color: scheme.surfaceContainer,
                child: Padding(
                  padding: const EdgeInsets.all(20),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '订阅到期时间',
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                      const SizedBox(height: 10),
                      Row(
                        crossAxisAlignment: CrossAxisAlignment.end,
                        children: [
                          Expanded(
                            child: Text(
                              _formatDate(expireDate),
                              style: Theme.of(context).textTheme.headlineSmall
                                  ?.copyWith(fontWeight: FontWeight.w700),
                            ),
                          ),
                          const SizedBox(width: 16),
                          Column(
                            crossAxisAlignment: CrossAxisAlignment.end,
                            children: [
                              Text(
                                pending
                                    ? '待开通'
                                    : expired
                                    ? '已过期'
                                    : '剩余 $daysLeft 天',
                                style: Theme.of(context).textTheme.titleMedium
                                    ?.copyWith(
                                      color: statusColor,
                                      fontWeight: FontWeight.w700,
                                    ),
                              ),
                            ],
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 14),
              Card(
                elevation: 0,
                margin: EdgeInsets.zero,
                color: scheme.surfaceContainer,
                child: Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 20,
                    vertical: 8,
                  ),
                  child: Column(
                    children: [
                      _infoRow(
                        Icons.cloud_done_outlined,
                        '订阅状态',
                        _subscriptionStatus(profile),
                      ),
                      Divider(height: 1, color: scheme.outlineVariant),
                      _infoRow(
                        Icons.data_usage_outlined,
                        '已用流量',
                        _formatTraffic(user['trafficUsed']),
                      ),
                      Divider(height: 1, color: scheme.outlineVariant),
                      _infoRow(
                        Icons.storage,
                        '总流量',
                        _formatTraffic(user['trafficTotal']),
                      ),
                      Divider(height: 1, color: scheme.outlineVariant),
                      _infoRow(
                        Icons.update,
                        '最近同步',
                        profile?.lastUpdateDate == null
                            ? '尚未同步'
                            : _formatDateTime(profile!.lastUpdateDate!),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 18),
              FilledButton.icon(
                onPressed: _syncing ? null : _syncSubscription,
                icon: _syncing
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Icon(Icons.sync),
                label: Text(_syncing ? '正在检测…' : '立即检测并导入'),
              ),
              const SizedBox(height: 10),
              OutlinedButton.icon(
                onPressed: _syncing ? null : _switchChannel,
                icon: const Icon(Icons.swap_horiz),
                label: Text(
                  authSession.usingBackupChannel ? '备用官网通道' : '主官网通道',
                ),
              ),
              const SizedBox(height: 10),
              OutlinedButton.icon(
                onPressed: _syncing ? null : _showServerSettings,
                icon: const Icon(Icons.dns_outlined),
                label: Text(
                  authSession.hasCustomServer ? '服务器地址（自定义）' : '设置服务器地址',
                ),
              ),
              const SizedBox(height: 14),
              Text(
                '每次启动软件时自动检测官网订阅；检测到账号、订阅链接或订阅内容更新后，会自动导入最新订阅。',
                style: Theme.of(
                  context,
                ).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant),
              ),
            ],
          ),
        );
      },
    );
  }
}
