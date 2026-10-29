import 'dart:convert';

import 'package:duanju_app/audiobook_fanqie.dart';
import 'package:duanju_app/core_bridge.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('搜索只保留听书，并去掉高亮标签', () {
    final page = fanqieSearchPage(
      jsonDecode('''
{"code":0,"data":{"has_more":true,"ret_data":[
 {"book_id":"1","title":"<em>评书</em>水浒","author":"甲","category":"评书","genre":"4"},
 {"book_id":"2","title":"文字小说","author":"乙","category":"玄幻","genre":"0"}
]}}
'''),
    );
    expect(page.books.map((book) => book.title), ['评书水浒']);
    expect(page.books.single.author, '甲');
    expect(page.hasMore, isTrue);
  });

  test('听书榜只保留有声书', () {
    final page = fanqieRankPage(
      jsonDecode('''
{"code":0,"data":{"has_more":false,"result":[
 {"book_id":"9","book_name":"十日终焉","author":"甲","category":"悬疑","genre":"4"},
 {"book_id":"8","book_name":"文字小说","author":"乙","category":["玄幻"],"genre":"0"}
]}}
'''),
    );
    expect(page.books.single.title, '十日终焉');
    expect(page.books.single.category, '悬疑');
    expect(page.hasMore, isFalse);
  });

  test('章节原文按段切开，并按字数摊到时长上', () {
    final texts = fanqieChapterTexts(
      jsonDecode(
        '{"data":{"chapterData":{"content":"<p>甲乙</p><p idx=\'1\'>丙丁戊</p><p></p>"}}}',
      ),
    );
    expect(texts, ['甲乙', '丙丁戊']);
    final lines = paceAudiobookLines(texts, 10);
    expect(lines.first.start, 0);
    expect(lines.first.end, closeTo(4, 0.01));
    expect(lines.last.end, 10);
    expect(audiobookLineAt(lines, 0), 0);
    expect(audiobookLineAt(lines, 4), 1);
    expect(audiobookLineAt(lines, 9.9), 1);
  });

  test('目录按页切开，并标出锁定章节', () async {
    final all = fanqieChapters(
      jsonDecode('''
{"data":{"chapterListWithVolume":[[
 {"itemId":"9","title":"第一章","realChapterOrder":"1","needPay":0,"isChapterLock":false},
 {"itemId":"10","title":"第二章","realChapterOrder":"2","needPay":1,"isChapterLock":false}
]]}}
'''),
    );
    expect(all, hasLength(2));
    expect(all.last.locked, isTrue);
    final api = FanqieAudiobookApi(
      fetcher: (url) async {
        if (url.contains('directory')) {
          final rows = [
            for (var index = 1; index <= 60; index++)
              {
                'itemId': '$index',
                'title': '第$index章',
                'realChapterOrder': '$index',
                'needPay': 0,
                'isChapterLock': false,
              },
          ];
          return jsonEncode({
            'data': {
              'chapterListWithVolume': [rows],
            },
          });
        }
        return '';
      },
    );
    final first = await api.chapters('1');
    expect(first.chapters, hasLength(50));
    expect(first.hasMore, isTrue);
    final second = await api.chapters('1', page: 2);
    expect(second.chapters.first.order, 51);
    expect(second.chapters, hasLength(10));
    expect(second.hasMore, isFalse);
    final full = await api.allChapters('1');
    expect(full, hasLength(60));
    await expectLater(
      api.requirePlayable('1', '1'),
      throwsA(isA<AppFailure>()),
    );
  });
}
