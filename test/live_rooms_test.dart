import 'dart:convert';

import 'package:duanju_app/live_http.dart';
import 'package:duanju_app/live_models.dart';
import 'package:duanju_app/live_repository.dart';
import 'package:duanju_app/live_rooms.dart';
import 'package:duanju_app/live_sources.dart';
import 'package:flutter_test/flutter_test.dart';

class _RouteHttp extends LiveHttp {
  _RouteHttp(this.routes);

  final Map<String, String> routes;
  final List<String> requested = [];

  @override
  Future<String> get(
    String url, {
    Map<String, String> headers = const {},
  }) async {
    requested.add(url);
    for (final entry in routes.entries) {
      if (url.contains(entry.key)) return entry.value;
    }
    throw StateError('未合成的请求：$url');
  }
}

class _DouyuHttp extends LiveHttp {
  final List<String> posts = [];

  @override
  Future<LiveHttpResult> getResult(
    String url, {
    Map<String, String> headers = const {},
  }) async => LiveHttpResult(
    jsonEncode({
      'error': 0,
      'data': {
        'key': 'abc',
        'rand_str': 'xyz',
        'enc_time': 2,
        'enc_data': 'ZZ',
        'is_special': 0,
      },
    }),
    date: DateTime.utc(2023, 11, 14, 22, 13, 20),
  );

  @override
  Future<String> postForm(
    String url,
    String body, {
    Map<String, String> headers = const {},
  }) async {
    posts.add(body);
    final rate = RegExp(r'(?:^|&)rate=([^&]*)').firstMatch(body)?.group(1);
    return jsonEncode({
      'error': 0,
      'data': {
        'rtmp_url': 'https://cdn.example',
        'rtmp_live': rate == '8' ? 'high.flv?token=1' : 'low.flv?token=1',
        'multirates': [
          {'name': '原画', 'rate': 8, 'bit': 9000},
          {'name': '高清', 'rate': 0, 'bit': 100},
        ],
      },
    });
  }
}

void main() {
  test('哔哩哔哩同档优先 flv，更高 current_qn 优先于封装', () {
    final same = bilibiliBestPlay({
      'data': {
        'playurl_info': {
          'playurl': {
            'stream': [
              {
                'protocol_name': 'http_hls',
                'format': [
                  {
                    'format_name': 'ts',
                    'codec': [
                      {
                        'current_qn': 250,
                        'base_url': '/low.m3u8?',
                        'url_info': [
                          {'host': 'https://hls.example', 'extra': 'qn=250'},
                        ],
                      },
                    ],
                  },
                ],
              },
              {
                'protocol_name': 'http_stream',
                'format': [
                  {
                    'format_name': 'flv',
                    'codec': [
                      {
                        'current_qn': 250,
                        'base_url': '/low.flv?',
                        'url_info': [
                          {'host': 'https://flv.example', 'extra': 'qn=250'},
                        ],
                      },
                    ],
                  },
                ],
              },
            ],
          },
        },
      },
    });
    expect(same?.url, 'https://flv.example/low.flv?qn=250');

    final higher = bilibiliBestPlay({
      'data': {
        'playurl_info': {
          'playurl': {
            'stream': [
              {
                'protocol_name': 'http_stream',
                'format': [
                  {
                    'format_name': 'flv',
                    'codec': [
                      {
                        'current_qn': 250,
                        'base_url': '/low.flv?',
                        'url_info': [
                          {'host': 'https://flv.example', 'extra': 'qn=250'},
                        ],
                      },
                    ],
                  },
                ],
              },
              {
                'protocol_name': 'http_hls',
                'format': [
                  {
                    'format_name': 'ts',
                    'codec': [
                      {
                        'current_qn': 10000,
                        'base_url': '/high.m3u8?',
                        'url_info': [
                          {'host': 'https://hls.example', 'extra': 'qn=10000'},
                        ],
                      },
                    ],
                  },
                ],
              },
            ],
          },
        },
      },
    });
    expect(higher?.url, 'https://hls.example/high.m3u8?qn=10000');
    expect(higher?.quality, 10000);
  });

  test('斗鱼鉴权与最高码率档', () {
    expect(
      douyuAuth(
        roomId: '1',
        timestamp: 1700000000,
        key: 'abc',
        randStr: 'xyz',
        encTime: 2,
        special: 0,
      ),
      '913772d0320c2fb6eec7d1e861863667',
    );
    expect(
      douyuAuth(
        roomId: '1',
        timestamp: 1700000000,
        key: 'abc',
        randStr: 'xyz',
        encTime: 2,
        special: 1,
      ),
      '80c8483398fcec5e8878fca2de7830c8',
    );
    expect(
      douyuPreferredRate([
        {'rate': 0, 'bit': 15171},
        {'rate': 4, 'bit': 4000},
      ]),
      0,
    );
    expect(
      douyuPreferredRate([
        {'rate': 8, 'bit': 9000},
        {'rate': 0, 'bit': 100},
      ]),
      8,
    );
    expect(
      douyuPreferredRate([
        {'rate': 2, 'bit': 0},
        {'rate': 4, 'bit': 4000},
      ]),
      2,
      reason: 'bit 为 0 的档位按原画处理',
    );
    expect(
      douyuPlayUrl({
        'rtmp_url': 'https://cdn.example',
        'rtmp_live': 'a.flv?x=1',
      }),
      'https://cdn.example/a.flv?x=1',
    );
  });

  test('虎牙优先主线路，否则取优先级更高的原画', () {
    expect(
      huyaBestFlv({
        'baseSteamInfoList': [
          {
            'sFlvUrl': 'http://a.example/src',
            'sStreamName': 'low',
            'sFlvUrlSuffix': 'flv',
            'sFlvAntiCode': 'a=1',
            'iIsMaster': 0,
            'iPCPriorityRate': 50,
          },
          {
            'sFlvUrl': 'http://b.example/src',
            'sStreamName': 'high',
            'sFlvUrlSuffix': 'flv',
            'sFlvAntiCode': 'b=1',
            'iIsMaster': 1,
            'iPCPriorityRate': -1,
          },
        ],
      }),
      'http://b.example/src/high.flv?b=1',
    );
    expect(
      huyaBestFlv({
        'baseSteamInfoList': [
          {
            'sFlvUrl': 'http://a.example/src',
            'sStreamName': 'slow',
            'sFlvUrlSuffix': 'flv',
            'sFlvAntiCode': 'a=1',
            'iIsMaster': 0,
            'iPCPriorityRate': -1,
          },
          {
            'sFlvUrl': 'http://c.example/src',
            'sStreamName': 'fast',
            'sFlvUrlSuffix': 'flv',
            'sFlvAntiCode': 'c=1',
            'iIsMaster': 0,
            'iPCPriorityRate': 50,
          },
        ],
      }),
      'http://c.example/src/fast.flv?c=1',
    );
  });

  test('哔哩哔哩房间播放地址选用返回结果中的最高清晰度', () async {
    final http = _RouteHttp({
      'getRoomPlayInfo': jsonEncode({
        'code': 0,
        'data': {
          'playurl_info': {
            'playurl': {
              'stream': [
                {
                  'protocol_name': 'http_stream',
                  'format': [
                    {
                      'format_name': 'flv',
                      'codec': [
                        {
                          'current_qn': 250,
                          'base_url': '/low.flv?',
                          'url_info': [
                            {'host': 'https://flv.example', 'extra': 'qn=250'},
                          ],
                        },
                        {
                          'current_qn': 10000,
                          'base_url': '/high.flv?',
                          'url_info': [
                            {
                              'host': 'https://flv.example',
                              'extra': 'qn=10000',
                            },
                          ],
                        },
                      ],
                    },
                  ],
                },
              ],
            },
          },
        },
      }),
      'getRoomList': jsonEncode({
        'code': 0,
        'data': {
          'list': [
            {
              'roomid': 7,
              'title': '总决赛',
              'uname': '主播',
              'online': 12,
              'user_cover': 'https://img.example/a.jpg',
            },
          ],
        },
      }),
      'Area/getList': jsonEncode({
        'code': 0,
        'data': [
          {'id': 2, 'name': '网游'},
        ],
      }),
    });
    final repository = LiveRepository(http: http);
    addTearDown(repository.dispose);
    final source = LiveSource.byId('bilibili');
    expect(source, isNotNull);

    final platforms = await repository.platforms(source!);
    expect(platforms.map((item) => item.name), ['网游']);
    final channels = await repository.channels(source, platforms.first);
    expect(channels.single.token, '7');
    expect(channels.single.name, '主播');
    expect(channels.single.subtitle, '总决赛 · 在线 12');

    final plan = await repository.playback(source, channels.single);
    expect(plan.url, 'https://flv.example/high.flv?qn=10000');
    expect(plan.headers['Referer'], bilibiliReferer);
    expect(
      http.requested.where((url) => url.contains('getRoomPlayInfo')).single,
      contains('qn=10000'),
    );
  });

  test('斗鱼收藏房间仍按房间所属源取最高档', () async {
    final http = _DouyuHttp();
    final repository = LiveRepository(http: http);
    addTearDown(repository.dispose);
    const other = LiveSource(
      id: 'other',
      name: '其他',
      description: '合成',
      protocol: LiveProtocol.pingtai,
    );
    final channel = LiveChannel(
      key: LiveChannel.makeKey('douyu', '英雄联盟', '1'),
      name: '总决赛',
      urls: const [],
      group: '英雄联盟',
      source: 'douyu',
      token: '1',
    );

    final plan = await repository.playback(other, channel);

    expect(plan.url, 'https://cdn.example/high.flv?token=1');
    expect(plan.headers['Referer'], douyuReferer);
    expect(http.posts, hasLength(2));
    expect(http.posts.first, contains('rate=0'));
    expect(http.posts.first, contains('tt=1700000000'));
    expect(http.posts.first, contains('auth=913772d0320c2fb6eec7d1e861863667'));
    expect(http.posts.last, contains('rate=8'));
  });

  test('虎牙隐藏空分区，翻过最后一页得到空列表', () async {
    final http = _RouteHttp({
      'page=2': jsonEncode({
        'status': 200,
        'data': {'datas': <Object>[]},
      }),
      'profileRoom': jsonEncode({
        'status': 200,
        'data': {
          'stream': {
            'baseSteamInfoList': [
              {
                'sFlvUrl': 'http://cdn.example/src',
                'sStreamName': 'room',
                'sFlvUrlSuffix': 'flv',
                'sFlvAntiCode': 'ws=1',
                'iIsMaster': 0,
                'iPCPriorityRate': 50,
              },
            ],
          },
        },
      }),
      'getLiveListByPage': jsonEncode({
        'status': 200,
        'data': {
          'datas': [
            {'profileRoom': '9', 'nick': '甲', 'roomName': '', 'totalCount': 3},
          ],
        },
      }),
      'getGameList': jsonEncode({
        'status': 200,
        'gameList': [
          {'gid': 1, 'gameFullName': '英雄联盟', 'profileNum': 3, 'isHide': 0},
          {'gid': 2, 'gameFullName': '隐藏', 'profileNum': 3, 'isHide': 1},
          {'gid': 3, 'gameFullName': '空', 'profileNum': 0, 'isHide': 0},
        ],
      }),
    });
    final repository = LiveRepository(http: http);
    addTearDown(repository.dispose);
    final source = LiveSource.byId('huya')!;

    final platforms = await repository.platforms(source);
    expect(platforms.map((item) => item.name), ['英雄联盟']);
    final channels = await repository.channels(source, platforms.first);
    expect(channels.single.name, '甲');
    expect(channels.single.token, '9');
    final plan = await repository.playback(source, channels.single);
    expect(plan.url, 'http://cdn.example/src/room.flv?ws=1');
    final next = await repository.channels(source, platforms.first, page: 2);
    expect(next, isEmpty);
  });
}
