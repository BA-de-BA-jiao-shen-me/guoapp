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
  StreamSubscription<PlayerLog>? logs;
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
    // 没有 VideoController 时 media_kit 默认 vid=no（见 media_kit NativePlayer._create）。
    // 冒烟片源是纯视频（remux 时只 map 了 0:v:0），音轨又设成 no，若不显式打开视频轨，
    // mpv 没有任何可播的流，时钟不会走：playing=true、buffering=true、position 永远 0。
    // 这里显式选 vid=auto，让 vo=null 真正解码视频帧并推进 time-pos。
    player = Player(
      configuration: const PlayerConfiguration(
        muted: true,
        vo: 'null',
        logLevel: MPVLogLevel.warn,
      ),
    );
    final native = player.platform;
    if (native is NativePlayer) {
      // 无头 runner 没有可用声卡。
      await native.setProperty('ao', 'null');
    }
    logs = player.stream.log.listen(
      (log) => stderr.writeln('mpv ${log.level} [${log.prefix}] ${log.text}'),
    );
    errors = player.stream.error.listen((message) {
      lastPlayerError = message;
      stderr.writeln('media_kit error: $message');
    });
    await player.setVideoTrack(VideoTrack.auto());
    await player.open(Media(remuxed.path), play: true);
    await player.setAudioTrack(AudioTrack.no());
    await player.stream.duration
        .firstWhere((time) => time.inMilliseconds >= 1000)
        .timeout(const Duration(seconds: 10));
    await player.stream.position
        .firstWhere((time) => time.inMilliseconds >= 400)
        .timeout(const Duration(seconds: 20));
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
    await logs.cancel();
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
    await logs?.cancel();
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
