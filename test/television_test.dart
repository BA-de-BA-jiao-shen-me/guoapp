import 'package:duanju_app/app_layout.dart';
import 'package:duanju_app/detail_screen.dart';
import 'package:duanju_app/local_store.dart';
import 'package:duanju_app/main.dart';
import 'package:duanju_app/models.dart';
import 'package:duanju_app/player_screen.dart';
import 'package:duanju_app/remote_widgets.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'fixtures.dart';
import 'remote_test_helpers.dart';

class TelevisionRepository extends FixtureRepository {
  @override
  Future<DramaDetail> detail(Drama drama) async {
    detailCalls++;
    return DramaDetail(
      drama,
      List.generate(
        100,
        (index) => Episode({
          'id': '${index + 1}',
          'currentEpisode': index + 1,
          'vip': true,
        }, index + 1),
      ),
    );
  }
}

void main() {
  Future<LocalStore> makeStore() async {
    SharedPreferences.setMockInitialValues({});
    final store = LocalStore(await SharedPreferences.getInstance());
    addTearDown(store.dispose);
    return store;
  }

  void size(WidgetTester tester, Size size) {
    tester.view.physicalSize = size;
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
  }

  Future<void> press(WidgetTester tester, LogicalKeyboardKey key) async {
    if (key == LogicalKeyboardKey.goBack) {
      await tester.binding.handlePopRoute();
    } else {
      await tester.sendKeyEvent(key);
    }
    await tester.pumpAndSettle();
  }

  testWidgets(
    'Android TV detection falls back safely when the platform channel is unavailable',
    (tester) async {
      debugDefaultTargetPlatformOverride = TargetPlatform.android;
      addTearDown(() => debugDefaultTargetPlatformOverride = null);
      final messenger =
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
      addTearDown(
        () => messenger.setMockMethodCallHandler(AppDevice.channel, null),
      );
      messenger.setMockMethodCallHandler(AppDevice.channel, (call) async {
        expect(call.method, 'deviceInfo');
        return {'television': true, 'version': appVersion};
      });
      final detected = await AppDevice.detect();
      expect(detected.television, isTrue);
      expect(detected.version, appVersion);
      messenger.setMockMethodCallHandler(
        AppDevice.channel,
        (call) async => throw MissingPluginException(),
      );
      expect((await AppDevice.detect()).television, isFalse);
      debugDefaultTargetPlatformOverride = null;
    },
  );

  testWidgets(
    'manual interface preference overrides device detection and survives restart',
    (tester) async {
      size(tester, const Size(960, 540));
      final store = await makeStore();
      final repository = FixtureRepository();
      await tester.pumpWidget(
        DuanjuApp(repository: repository, store: store, television: true),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('tv-nav-0')), findsOneWidget);
      await tester.tap(find.byTooltip('更多'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('界面模式'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('手机 / 电脑'));
      await tester.pumpAndSettle();
      expect(store.displayMode, 'standard');
      expect(find.byKey(const ValueKey('tv-nav-0')), findsNothing);
      expect(LocalStore(store.preferences).displayMode, 'standard');
      await store.setDisplayMode('television');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('tv-nav-0')), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  for (final dimensions in [const Size(960, 540), const Size(1280, 720)]) {
    testWidgets(
      'TV sources, details, VIP confirmation and back retain remote focus at $dimensions',
      (tester) async {
        size(tester, dimensions);
        final store = await makeStore();
        final repository = TelevisionRepository();
        await tester.pumpWidget(
          DuanjuApp(repository: repository, store: store, television: true),
        );
        await tester.pumpAndSettle();
        // 首页切换的是站源分组（SourceGroup），同组多个站源合并显示，按分组顺序取第二项。
        final groups = SourceGroup.fromSources(store.sources);
        if (groups.length > 1) {
          final switcher = find.byKey(const ValueKey('source-switch'));
          final title = find
              .descendant(of: switcher, matching: find.byType(Text))
              .first;
          Focus.of(tester.element(title)).requestFocus();
          await tester.pumpAndSettle();
          await press(tester, LogicalKeyboardKey.select);
          // 菜单首项是聚合的“全部”，其后才是各分组，因此下移三次落到第二个分组。
          await press(tester, LogicalKeyboardKey.arrowDown);
          await press(tester, LogicalKeyboardKey.arrowDown);
          await press(tester, LogicalKeyboardKey.arrowDown);
          await press(tester, LogicalKeyboardKey.select);
        }
        expect(
          repository.requests.last,
          groups.length > 1 ? groups[1].sources.first.id : 'hongguo',
        );
        focusRemote(tester, find.byKey(ValueKey(FixtureRepository.free.id)));
        await tester.pumpAndSettle();
        await press(tester, LogicalKeyboardKey.select);
        // 0.2.48 起点剧在当前页读取详情后直接进入播放，不再先进详情页；
        // 本仓库的剧集全是 VIP，所以先弹 VIP 确认，返回后焦点回到卡片。
        expect(repository.detailCalls, 1);
        expect(find.text('这是一集 VIP 内容'), findsOneWidget);
        await press(tester, LogicalKeyboardKey.goBack);
        expect(find.text('这是一集 VIP 内容'), findsNothing);
        expect(find.byType(PlayerScreen), findsNothing);
        expect(
          FocusManager.instance.primaryFocus?.debugLabel,
          'remote-${FixtureRepository.free.id}',
        );
        // 详情页的 VIP 确认与返回焦点单独在详情页上验证。
        await tester.pumpWidget(
          televisionHost(
            child: DetailScreen(
              drama: FixtureRepository.free,
              repository: repository,
              store: store,
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byKey(const ValueKey('start-play')), findsOneWidget);
        focusRemote(tester, find.byKey(const ValueKey('episode-1')));
        await tester.pumpAndSettle();
        await press(tester, LogicalKeyboardKey.select);
        expect(find.text('这是一集 VIP 内容'), findsOneWidget);
        await press(tester, LogicalKeyboardKey.goBack);
        expect(find.text('这是一集 VIP 内容'), findsNothing);
        expect(FocusManager.instance.primaryFocus?.debugLabel, 'remote-1');
        expect(tester.takeException(), isNull);
      },
    );
  }

  testWidgets(
    'remote grid reaches unbuilt rows, partial last row and pagination without touch',
    (tester) async {
      size(tester, const Size(960, 540));
      var loaded = 0;
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: RemoteGrid(
              itemKeys: List.generate(59, (index) => '$index'),
              columns: 4,
              itemExtent: 100,
              autofocus: true,
              footer: Center(
                child: RemoteButton(label: '加载更多', onPressed: () => loaded++),
              ),
              itemBuilder: (_, index, node, onFocus) => RemoteEpisodeButton(
                number: index,
                focusNode: node,
                onFocus: onFocus,
                onPressed: () {},
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(FocusManager.instance.primaryFocus?.debugLabel, 'remote-0');
      for (var index = 0; index < 3; index++) {
        await press(tester, LogicalKeyboardKey.arrowRight);
      }
      for (var row = 0; row < 14; row++) {
        await press(tester, LogicalKeyboardKey.arrowDown);
      }
      expect(FocusManager.instance.primaryFocus?.debugLabel, 'remote-58');
      await press(tester, LogicalKeyboardKey.arrowDown);
      await press(tester, LogicalKeyboardKey.select);
      expect(loaded, 1);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('episode grid initially focuses an offscreen saved episode', (
    tester,
  ) async {
    size(tester, const Size(960, 540));
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: RemoteGrid(
            itemKeys: List.generate(160, (index) => '$index'),
            columns: 6,
            itemExtent: 64,
            initialIndex: 131,
            autofocus: true,
            itemBuilder: (_, index, node, onFocus) => RemoteEpisodeButton(
              number: index,
              focusNode: node,
              onFocus: onFocus,
              onPressed: () {},
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(FocusManager.instance.primaryFocus?.debugLabel, 'remote-131');
    await press(tester, LogicalKeyboardKey.arrowUp);
    expect(FocusManager.instance.primaryFocus?.debugLabel, 'remote-125');
    expect(tester.takeException(), isNull);
  });
}
