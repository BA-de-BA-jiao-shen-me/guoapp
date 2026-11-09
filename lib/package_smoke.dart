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
    // CI Windows runner has no usable audio device; remux also keeps video only.
    // Without ao=null, libmpv can sit idle and never advance time-pos.
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
    }
    errors = player.stream.error.listen((message) {
      lastPlayerError = message;
      stderr.writeln('media_kit error: $message');
    });
    final advancing = player.stream.position.firstWhere(
      (time) => time.inMilliseconds >= 400,
    );
    await player.open(Media(remuxed.path), play: true);
    await player.setAudioTrack(AudioTrack.no());
    await advancing.timeout(const Duration(seconds: 20));
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
