import 'dart:convert';
import 'dart:io';

import 'package:duanju_app/audiobook_fanqie.dart';
import 'package:duanju_app/audiobook_screen.dart';
import 'package:duanju_app/audiobook_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:media_kit/media_kit.dart';

import 'fixtures.dart';
import 'player_fixtures.dart';

class _AudioRepository extends FixtureRepository {
  final mediaRequests = <String>[];

  @override
  Future<Map<String, dynamic>> audiobookMedia(
    String bookId,
    String chapterId,
  ) async {
    mediaRequests.add(chapterId);
    return {
      'url': 'https://audio.test/$bookId/$chapterId.m4a',
      'decryptionKey': '',
      'duration': 120,
      'headers': const <String, String>{},
    };
  }
}

const _book = FanqieBook(
  id: 'b1',
  title: '测试有声书',
  author: '作者',
  category: '悬疑',
);

String _directory() => jsonEncode({
  'data': {
    'chapterListWithVolume': [
      [
        for (var i = 1; i <= 6; i++)
          {'itemId': 'c$i', 'title': '第$i章', 'realChapterOrder': '$i'},
      ],
    ],
  },
});

void main() {
  late Directory temp;
  late ScriptedPlayer scripted;

  setUp(() {
    temp = Directory.systemTemp.createTempSync('audiobook-select');
    AudiobookSession.debugPlayerFactory = () {
      scripted = ScriptedPlayer();
      return Player(platformPlayer: scripted);
    };
  });

  tearDown(() {
    AudiobookSession.debugPlayerFactory = null;
    temp.deleteSync(recursive: true);
  });

  Future<_AudioRepository> pumpCatalog(WidgetTester tester) async {
    // 会话是全应用单例：每个用例从干净状态开始，互不影响。
    await resetSession(tester);
    final repository = _AudioRepository();
    final store = (await tester.runAsync(
      () => AudiobookStore.openDirectory(temp),
    ))!;
    final api = FanqieAudiobookApi(fetcher: (url) async => _directory());
    await tester.pumpWidget(
      MaterialApp(
        home: AudiobookCatalogScreen(
          api: api,
          repository: repository,
          store: store,
          book: _book,
        ),
      ),
    );
    await settle(tester);
    return repository;
  }

  testWidgets('目录里点第 4 章，打开并播放的就是第 4 章', (tester) async {
    final repository = await pumpCatalog(tester);
    await tester.tap(find.byKey(const ValueKey('audiobook-chapter-c4')));
    await settle(tester);
    await waitForChapter(tester, 'c4');
    expect(repository.mediaRequests, ['c4']);
    expect(scriptedUri(), contains('/c4.m4a'));
    expect(AudiobookSession.instance.chapter?.id, 'c4');
    await resetSession(tester);
  });

  testWidgets('正在听第 1 章时回目录点第 5 章，切到第 5 章', (tester) async {
    final repository = await pumpCatalog(tester);
    await tester.tap(find.byKey(const ValueKey('audiobook-chapter-c1')));
    await settle(tester);
    await waitForChapter(tester, 'c1');
    await tester.pageBack();
    await tester.pumpAndSettle();
    await settle(tester);
    await tester.tap(find.byKey(const ValueKey('audiobook-chapter-c5')));
    await settle(tester);
    await waitForChapter(tester, 'c5');
    expect(repository.mediaRequests.last, 'c5');
    expect(scriptedUri(), contains('/c5.m4a'));
    expect(AudiobookSession.instance.chapter?.id, 'c5');
    await resetSession(tester);
  });
}

/// 停掉全局会话。脚本播放器的 dispose 不一定结束，状态在 dispose 之前已经清空，
/// 所以只等一小会儿。
Future<void> resetSession(WidgetTester tester) => tester.runAsync(
  () => AudiobookSession.instance.stop().timeout(
    const Duration(milliseconds: 200),
    onTimeout: () {},
  ),
);

Future<void> settle(WidgetTester tester) async {
  for (var i = 0; i < 20; i++) {
    await tester.runAsync(() => Future<void>.delayed(Duration.zero));
    await tester.pump(const Duration(milliseconds: 50));
  }
}

/// 等到目标章真正 ready，且脚本播放器已经 open 到对应音频。
/// 只看 chapter.id 不够：start() 一改 index 就会变，open/player.open 可能还没跑完。
Future<void> waitForChapter(WidgetTester tester, String id) async {
  for (var i = 0; i < 80; i++) {
    final session = AudiobookSession.instance;
    final opened = scriptedUri();
    if (session.chapter?.id == id &&
        session.ready &&
        opened != null &&
        opened.contains('/$id.m4a')) {
      return;
    }
    await tester.runAsync(() => Future<void>.delayed(Duration.zero));
    await tester.pump(const Duration(milliseconds: 50));
  }
  final session = AudiobookSession.instance;
  expect(session.chapter?.id, id, reason: 'chapter.id');
  expect(session.ready, isTrue, reason: 'session.ready');
  expect(scriptedUri(), contains('/$id.m4a'), reason: 'opened uri');
}

String? scriptedUri() {
  // debugPlayerFactory 每次新建都会覆盖测试里的 scripted 引用；
  // 这里从会话里的真实 platform 读，避免拿到旧实例。
  final platform = AudiobookSession.instance.player.platform;
  if (platform is ScriptedPlayer && platform.opened.isNotEmpty) {
    return platform.opened.last.uri;
  }
  return null;
}
