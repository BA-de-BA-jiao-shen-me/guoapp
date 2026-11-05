import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import 'app_build.dart';

/// 全部站源构建安装后先显示默认桌面图标，只有激活分级限制开关才换成
/// 完整模式入口；默认构建不提供切换，桌面入口始终只有一种。
class LauncherIcon {
  static const _channel = MethodChannel('duanju/device');

  static bool get supported =>
      allSourcesEnabled && !kIsWeb && Platform.isAndroid;

  static Future<void> apply(bool full) async {
    if (!supported) return;
    try {
      await _channel.invokeMethod<bool>('setLauncherEdition', {'full': full});
    } catch (_) {
      // 切换失败时保留当前入口：桌面图标属于系统状态，不影响应用内功能。
    }
  }
}
