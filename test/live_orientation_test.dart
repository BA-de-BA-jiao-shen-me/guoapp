import 'package:duanju_app/app_orientation.dart';
import 'package:duanju_app/live_sources.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('直播锁横屏，旋转到竖屏后锁竖屏', () {
    expect(
      AppOrientationController.orientations(
        television: false,
        fullscreen: true,
        aspectRatio: 16 / 9,
      ),
      [DeviceOrientation.landscapeLeft, DeviceOrientation.landscapeRight],
      reason: '直播流为 16:9，进入播放页即锁横屏',
    );
    expect(
      AppOrientationController.orientations(
        television: false,
        fullscreen: true,
        aspectRatio: 9 / 16,
      ),
      [DeviceOrientation.portraitUp, DeviceOrientation.portraitDown],
      reason: '旋转按钮切到竖屏后必须真正锁竖屏，否则一松手就被系统转回横屏',
    );
  });

  test('电视模式始终锁横屏，不参与旋转', () {
    expect(AppOrientationController.orientations(television: true), [
      DeviceOrientation.landscapeLeft,
      DeviceOrientation.landscapeRight,
    ]);
  });

  test('内置直播源是哔哩哔哩、斗鱼和虎牙', () {
    expect(LiveSource.byId('xiongguo'), isNull);
    expect(LiveSource.byId('xiuguo'), isNull);
    expect(LiveSource.values.map((source) => source.id), [
      'bilibili',
      'douyu',
      'huya',
    ]);
    expect(
      LiveSource.values.every((source) => source.protocol == LiveProtocol.room),
      isTrue,
    );
    expect(LiveSource.values.every((source) => source.adult), isFalse);
  });
}
