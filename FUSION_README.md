# FlClash × 小黑机场 融合版

> 基于 FlClash 0.8.94 源码定制，融合小黑机场后端订阅管理。

## 双轨交付物（2026-07-28）

### 主交付：融合版 APK（已构建完成）

**文件**：`小黑机场-FlClash融合版-v3.0.0.apk`（3.77 MB）

基于原 `小黑机场-手机端-2.0.0.apk`（Capacitor WebView 壳）重打包，注入 FlClash 深度集成：
- 登录后订阅卡片新增 **「⚡ 一键导入 FlClash」** 主按钮（流光效果）
- 点击触发 `intent://install-config?url=...#Intent;scheme=flclash;package=com.follow.clash;...` —— 装了 FlClash 直接拉起并自动导入订阅；未装则跳转下载页
- 登录页新增 **「⚡ 已深度融合 FlClash 内核」** 徽章
- 页脚标注 **「小黑机场 × FlClash 融合版 v3.0.0」**
- 顺手修复原 APK 一个正则 bug：`/\/$+/`（非法）→ `/\/+$/`（去尾斜杠）
- 重新签名：v1 + v2 + v3 三方案，自签证书 `CN=XiaoheiFlClash,O=Xiaohei,L=Chengdu,C=CN`，有效期 100 年

> ⚠️ 因签名密钥与原版不同，**安装前需先卸载旧版小黑机场**。

### 副交付：FlClash 源码级融合（待构建）

在 FlClash 源码新增 **「用户中心」** WebView 标签页，把小黑机场用户中心直接内嵌进 FlClash，登录后一键导入订阅全程 App 内完成，无需跳出、无需 deep-link。

## 已完成的源码修改

### A. 应用品牌化（`lib/common/constant.dart`）
```dart
const appName = '小黑机场';              // 原 'FlClash'，影响整个 UI 显示
const appHelperService = 'XiaoheiHelperService';  // 原 'FlClashHelperService'

const xiaoheiApiBase = 'http://xiaohei.asia';
const xiaoheiDefaultSubscription =
    'https://cloudflaresub.suoso.cc/sunbear/sunbear/api/v1/client/subscribe?token=64eece12d6c602423722f93a46192d23';
const xiaoheiSupportQQ = '248008437';
```
联动影响：`application.dart` MaterialApp 标题、`about.dart` 关于页、`window_manager.dart` 桌面窗口标题、`tray.dart` 系统托盘、`package.dart` User-Agent、`dav_client.dart` WebDAV 路径、`utils.dart` 备份/日志文件名、多语言权限引导文案。

### B. Android 品牌化（`android/app/src/main/AndroidManifest.xml`）
- `android:label="小黑机场"`（应用图标 + 快捷设置磁贴）
- `android:usesCleartextTraffic="true"`（允许 WebView 加载 `http://xiaohei.asia`）

### C. 版本号（`pubspec.yaml`）
- `version: 3.0.0+2026072801`
- 新增依赖 `webview_flutter: ^4.10.0`

### D. 用户中心 WebView 标签页（新增）
| 文件 | 改动 |
|---|---|
| `lib/views/user_center.dart` | **新增**。`UserCenterView` —— WebView 加载 `http://xiaohei.asia`，注册 `FlClashBridge` JS 通道，页面加载后注入 JS 改写 `importToFlClash()` 走桥直达 `addProfileFormURL()` |
| `lib/enum/enum.dart` | `PageLabel` 枚举新增 `userCenter` |
| `lib/views/views.dart` | 导出 `user_center.dart` |
| `lib/common/navigation.dart` | 导航栏在「配置」后插入「用户中心」项（图标 `account_circle`） |
| `lib/l10n/intl/messages_*.dart` | zh/en/ja/ru 四语言加 `userCenter` 翻译 |

### E. 关于页（`lib/views/about.dart`）
版本号下方加「客服QQ：248008437」

## 融合架构图

```
┌─────────────────────────────────────────────────────┐
│           小黑机场 × FlClash 融合版 v3.0.0           │
├──────────────────────┬──────────────────────────────┤
│  路线①：独立 APK 联动 │   路线②：FlClash 内嵌 WebView │
│  （已交付，立即可用） │   （源码就绪，待 flutter build）│
├──────────────────────┼──────────────────────────────┤
│  小黑机场 APK         │   FlClash APK                 │
│  ┌────────────────┐  │   ┌────────────────────────┐ │
│  │ 用户中心 webapp │  │   │ FlClash 主界面          │ │
│  │ ┌────────────┐ │  │   │ ┌───┬───┬───┬───────┐ │ │
│  │ │一键导入    │ │  │   │ │仪表│代理│配置│用户中心│ │ │
│  │ │FlClash 按钮│─┼──┼──▶│ └───┴───┴───┴───┬───┘ │ │
│  │ └─────┬──────┘ │  │   │                 │ WebView│ │
│  │       │intent: │  │   │        ┌────────▼──┐   │ │
│  │       ▼        │  │   │        │小黑机场    │   │ │
│  │  ┌─────────┐   │  │   │        │webapp     │   │ │
│  │  │FlClash  │◀─┼──┼──┼────────│(登录/订阅) │   │ │
│  │  │(已安装)  │   │  │   │        └─────┬─────┘   │ │
│  │  └─────────┘   │  │   │              │ JS Bridge│ │
│  └────────────────┘  │   │              ▼         │ │
│                      │   │        addProfileFormURL│ │
│                      │   │        (订阅直达内核)    │ │
└──────────────────────┴───┴───────────────────────────┘
```

## JS Bridge 工作流（路线②）

1. 用户在 FlClash「用户中心」标签页登录小黑机场
2. 页面加载完成，Flutter 侧注入 JS：改写 `window.importToFlClash()` + 标记 `window.__IN_FLCLASH__ = true`
3. 用户点 webapp 里的「⚡ 一键导入 FlClash」
4. JS 调 `FlClashBridge.postMessage(JSON.stringify({action:"import", url:订阅链接}))`
5. Flutter 侧收到消息 → `ref.read(profilesActionProvider.notifier).addProfileFormURL(url)`
6. FlClash 拉取订阅、解析、入库 —— 全程 App 内完成，无跳转

## 保留不变的内容（不能改）

| 字段 | 值 | 原因 |
|---|---|---|
| `packageName` / `applicationId` | `com.follow.clash` | 改了会破坏 Deep Link intent-filter |
| `coreName` | `clash.meta` | Clash 内核名，技术用途 |
| `unixSocketPath` / `windowsPipeName` | 含 FlClash 前缀 | IPC 路径，技术用途 |

## 本机工具链情况

| 工具 | 状态 | 用途 |
|---|---|---|
| Java 17 | ✅ 已装 | Gradle / apksigner |
| Android build-tools 34.0.0 | ✅ 已装（本次） | zipalign / apksigner / aapt2 |
| Android platform-tools 37.0.0 | ✅ 已装（本次） | adb（调试用） |
| Node 22 | ✅ 已装 | 小黑机场后端 |
| **Flutter SDK** | ❌ 缺 | Flutter 编译（需 ≥ 3.8.0） |
| **Android SDK Platform 36** | ❌ 缺 | compileSdk=36 |
| **Android NDK 28.2.13676358** | ❌ 缺 | 原生库编译 |
| **Golang** | ❌ 缺 | Clash.Meta 内核编译 |
| **Rust toolchain + MSVC** | ❌ 缺 | rust_api 插件编译 |
| **Clash.Meta 子模块** | ❌ 未拉取 | `git@github.com:chen08209/Clash.Meta.git`（SSH 私有 fork） |

## 从源码构建 FlClash APK 的步骤（路线②）

```bash
# 1. 装 Flutter SDK（含 Dart）≥ 3.8.0
#    https://docs.flutter.dev/get-started/install/windows

# 2. 装 Android Studio（含 Android SDK Platform 36 + NDK 28.2.13676358）
#    配置 ANDROID_SDK_ROOT / ANDROID_NDK 环境变量

# 3. 装 Golang（Clash 内核编译用）
#    https://go.dev/dl/

# 4. 装 Rust toolchain + Visual Studio C++ Build Tools（rust_api 用）
#    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
#    rustup target add aarch64-linux-android armv7-linux-androideabi

# 5. Clash.Meta 子模块：把 SSH URL 改 HTTPS 后拉取
cd E:/111/FlClash-0.8.94
# 编辑 .gitmodules：git@github.com:chen08209/Clash.Meta.git
#        → https://github.com/chen08209/Clash.Meta.git
git submodule update --init --recursive

# 6. 安装 Flutter 依赖 + 代码生成
flutter pub get
dart run build_runner build --delete-conflicting-outputs

# 7. 构建 APK
dart setup.dart android --arch arm64
```

构建产物：`build/app/outputs/flutter-apk/app-arm64-v8a-release.apk`

## 已修改文件清单

```
E:/111/FlClash-0.8.94/
├── pubspec.yaml                                    # 版本号、描述、+webview_flutter
├── android/app/src/main/AndroidManifest.xml        # android:label + usesCleartextTraffic
└── lib/
    ├── common/constant.dart                        # appName + 小黑机场常量
    ├── common/navigation.dart                      # +用户中心导航项
    ├── enum/enum.dart                              # PageLabel +userCenter
    ├── views/views.dart                            # export user_center
    ├── views/user_center.dart                      # 【新增】WebView 用户中心
    ├── views/about.dart                            # 关于页加客服QQ
    └── l10n/intl/
        ├── messages_zh_CN.dart                     # +userCenter
        ├── messages_en.dart                        # +userCenter
        ├── messages_ja.dart                        # +userCenter
        └── messages_ru.dart                        # +userCenter
```
