import 'dart:async';

import 'package:duanju_app/player_screen.dart';
import 'package:media_kit/media_kit.dart';

import 'player_fixtures.dart';

/// 测试环境没有 libmpv：没显式传 playerFactory 的播放页一律用脚本播放器，
/// 避免 “MediaKit.ensureInitialized must be called” 异常。
Future<void> testExecutable(FutureOr<void> Function() testMain) async {
  PlayerScreen.debugPlayerFactory = () =>
      Player(platformPlayer: ScriptedPlayer());
  PlayerScreen.debugVideoBuilder = (controls) => controls;
  await testMain();
}
