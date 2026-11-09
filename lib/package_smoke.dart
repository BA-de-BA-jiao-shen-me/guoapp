import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:media_kit/media_kit.dart';

import 'core_bridge.dart';
import 'media_pipeline.dart';

Future<void> runPackageSmoke(List<String> arguments) async {
  if (arguments.length != 3) exit(2);
  final report = File(arguments[1]), input = arguments[2];
  Player? player;
  StreamSubscription<String>? errors;
  String? lastPlayerError;
  try {
    final repository = NativeRepository();
    await repository.initialize();
    final executor = FFmpegExecutor();
    final probe = await executor.probe(input);
    verifyMediaDuration(probe, 3);
    final remuxed = File(
      '${report.parent.path}${Platform.pathSeparator}remuxed.mkv',
    );
    await executor.run([
      '-i',
      input,
      '-map',
      '0:v:0',
      '-c',
      'copy',
      remuxed.path,
    ]);
    verifyMediaDuration(await executor.probe(remuxed.path), 3);
    // GitHub Windows runner 是无头/无可用声卡环境：ao/vo 都用 null，关掉硬解，
    // 并开 untimed，否则会出现 duration 已读到、playing=true，却一直 buffering、
    // time-pos 卡在 0 的情况。
    player = Player(
      configuration: const PlayerConfiguration(
        muted: true,
        vo: 'null',
        logLevel: MPVLogLevel.warn,
      ),
    );
    final native = player.platform;
    if (native is NativePlayer) {
      await native.setProperty('ao', 'null');
      await native.setProperty('hwdec', 'no');
      await native.setProperty('untimed', 'yes');
      await native.setProperty('video-sync', 'desync');
    }
    errors = player.stream.error.listen((message) {
      lastPlayerError = message;
      stderr.writeln('media_kit error: $message');
    });
    await player.open(Media(remuxed.path), play: true);
    await player.setAudioTrack(AudioTrack.no());
    await player.stream.duration
        .firstWhere((time) => time.inMilliseconds >= 1000)
        .timeout(const Duration(seconds: 10));
    await _waitForPlaybackPosition(player);
    await report.writeAsString(
      jsonEncode({
        'ok': true,
        'nativeCore': true,
        'ffprobe': true,
        'remux': true,
        'mediaKitPlayback': true,
      }),
      flush: true,
    );
    await errors.cancel();
    await player.dispose();
    exit(0);
  } catch (error) {
    final state = player?.state;
    final snapshot = state == null
        ? null
        : {
            'playing': state.playing,
            'positionMs': state.position.inMilliseconds,
            'durationMs': state.duration.inMilliseconds,
            'buffering': state.buffering,
            'completed': state.completed,
          };
    await errors?.cancel();
    await player?.dispose();
    await report.writeAsString(
      jsonEncode({
        'ok': false,
        'error': error.toString(),
        'playerError': ?lastPlayerError,
        'playerState': ?snapshot,
      }),
      flush: true,
    );
    exit(1);
  }
}

Future<void> _waitForPlaybackPosition(Player player) async {
  Future<void> untilAdvanced() => player.stream.position
      .firstWhere((time) => time.inMilliseconds >= 400)
      .timeout(const Duration(seconds: 12));
  try {
    await untilAdvanced();
  } on TimeoutException {
    // 无头环境下时钟仍可能不走：主动 seek 一次，确认解码管线能跳转。
    await player.seek(const Duration(milliseconds: 500));
    await untilAdvanced();
  }
}
