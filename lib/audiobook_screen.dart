import 'dart:async';
import 'dart:io';

import 'package:flutter/cupertino.dart' show CupertinoIcons;
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart' show RenderParagraph, ScrollDirection;
import 'package:flutter/services.dart';
import 'package:media_kit/media_kit.dart';

import 'audiobook_download.dart';
import 'audiobook_fanqie.dart';
import 'audiobook_store.dart';
import 'core_bridge.dart';
import 'widgets.dart';

const fanqieSearchWords = ['评书', '玄幻', '都市', '悬疑', '武侠'];

enum _AudiobookPane { search, shelf, history }

enum AudiobookSleep {
  off(0, '关'),
  quarter(15, '15分钟'),
  half(30, '30分钟'),
  hour(60, '60分钟'),
  long(90, '90分钟'),
  chapter(0, '本章结束');

  const AudiobookSleep(this.minutes, this.label);
  final int minutes;
  final String label;
}

class AudiobookScreen extends StatefulWidget {
  const AudiobookScreen({
    super.key,
    required this.repository,
    this.api,
    this.store,
  });

  final AppRepository repository;
  final FanqieAudiobookApi? api;
  final AudiobookStore? store;

  @override
  State<AudiobookScreen> createState() => _AudiobookScreenState();
}

class _AudiobookScreenState extends State<AudiobookScreen> {
  late final FanqieAudiobookApi _api = widget.api ?? FanqieAudiobookApi();
  final _search = TextEditingController();
  AudiobookStore? _store;
  _AudiobookPane _pane = _AudiobookPane.search;
  List<FanqieBook> _books = const [];
  bool _loading = false;
  bool _loadingMore = false;
  bool _hasMore = false;
  int _page = 1;
  String _query = '';
  FanqieAudiobookBoard? _board;
  String? _error;
  String? _storeError;

  @override
  void initState() {
    super.initState();
    _prepare();
    _loadBoard(fanqieAudiobookBoards.first);
  }

  @override
  void dispose() {
    _store?.removeListener(_onStore);
    _search.dispose();
    super.dispose();
  }

  Future<void> _prepare() async {
    try {
      final store = widget.store ?? await AudiobookStore.open();
      if (!mounted) return;
      store.addListener(_onStore);
      setState(() {
        _store = store;
        _storeError = null;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() => _storeError = '$error');
    }
  }

  void _onStore() {
    if (mounted) setState(() {});
  }

  Future<void> _loadBoard(
    FanqieAudiobookBoard board, {
    bool more = false,
  }) async {
    final page = more && _board == board && _query.isEmpty ? _page + 1 : 1;
    setState(() {
      if (page == 1) {
        _loading = true;
        _books = const [];
        _search.clear();
      } else {
        _loadingMore = true;
      }
      _error = null;
      _board = board;
      _query = '';
    });
    try {
      final result = await _api.rank(board, page: page);
      if (!mounted) return;
      setState(() {
        _books = page == 1 ? result.books : [..._books, ...result.books];
        _page = page;
        _hasMore = result.hasMore;
        _loading = false;
        _loadingMore = false;
        _error = page == 1 && result.books.isEmpty ? '听书榜暂时没有书' : null;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _loadingMore = false;
        if (_books.isEmpty) _error = '$error';
      });
      if (_books.isNotEmpty) _toast('$error');
    }
  }

  Future<void> _run([String? word, bool more = false]) async {
    final query = (word ?? _search.text).trim();
    if (word != null) _search.text = word;
    if (query.isEmpty) return;
    final page = more && query == _query ? _page + 1 : 1;
    setState(() {
      if (page == 1) {
        _loading = true;
        _books = const [];
      } else {
        _loadingMore = true;
      }
      _error = null;
      _board = null;
    });
    try {
      final result = await _api.search(query, page: page);
      if (!mounted) return;
      setState(() {
        _books = page == 1 ? result.books : [..._books, ...result.books];
        _page = page;
        _query = query;
        _hasMore = result.hasMore;
        _loading = false;
        _loadingMore = false;
        _error = page == 1 && result.books.isEmpty ? '没有找到听书' : null;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _loadingMore = false;
        if (_books.isEmpty) _error = '$error';
      });
      if (_books.isNotEmpty) _toast('$error');
    }
  }

  void _open(FanqieBook book, {bool resume = false}) {
    final store = _store;
    if (store == null) return;
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => AudiobookCatalogScreen(
          api: _api,
          repository: widget.repository,
          store: store,
          book: book,
          resumeOnOpen: resume,
        ),
      ),
    );
  }

  void _toast(String text) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  @override
  Widget build(BuildContext context) {
    final store = _store;
    if (_storeError != null) {
      return StatusPanel(
        title: '听书记录没有打开',
        message: _storeError!,
        icon: Icons.headphones_rounded,
        onRetry: _prepare,
      );
    }
    if (store == null) return const Center(child: CircularProgressIndicator());
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
          child: SegmentedButton<_AudiobookPane>(
            key: const ValueKey('audiobook-pane'),
            showSelectedIcon: false,
            segments: const [
              ButtonSegment(value: _AudiobookPane.search, label: Text('找书')),
              ButtonSegment(value: _AudiobookPane.shelf, label: Text('书架')),
              ButtonSegment(value: _AudiobookPane.history, label: Text('历史')),
            ],
            selected: {_pane},
            onSelectionChanged: (value) => setState(() => _pane = value.first),
          ),
        ),
        if (_pane == _AudiobookPane.search) ...[
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
            child: TextField(
              key: const ValueKey('audiobook-search'),
              controller: _search,
              textInputAction: TextInputAction.search,
              decoration: const InputDecoration(
                hintText: '搜索番茄畅听',
                prefixIcon: Icon(Icons.search_rounded),
              ),
              onSubmitted: (_) => _run(),
            ),
          ),
          SizedBox(
            height: 48,
            child: ListView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              children: [
                for (final board in fanqieAudiobookBoards)
                  Padding(
                    padding: const EdgeInsets.only(right: 8),
                    child: ChoiceChip(
                      label: Text(board.name),
                      selected: _query.isEmpty && _board == board,
                      onSelected: (_) => _loadBoard(board),
                    ),
                  ),
                for (final word in fanqieSearchWords)
                  Padding(
                    padding: const EdgeInsets.only(right: 8),
                    child: ChoiceChip(
                      label: Text(word),
                      selected: _query == word,
                      onSelected: (_) => _run(word),
                    ),
                  ),
              ],
            ),
          ),
        ],
        Expanded(child: _body(store)),
      ],
    );
  }

  Widget _body(AudiobookStore store) {
    switch (_pane) {
      case _AudiobookPane.shelf:
        if (store.shelf.isEmpty) {
          return const StatusPanel(
            title: '书架是空的',
            message: '打开一本书，在目录页加入书架。',
            icon: Icons.bookmark_border_rounded,
          );
        }
        return ListView.builder(
          itemCount: store.shelf.length,
          itemBuilder: (context, index) {
            final book = store.shelf[index];
            final latest = store.latest(book.id);
            return ListTile(
              key: ValueKey('audiobook-shelf-${book.id}'),
              title: Text(book.title),
              subtitle: Text(
                latest == null
                    ? _bookLine(book)
                    : '${latest.chapterTitle} · ${formatAudiobookClock(latest.seconds)}',
              ),
              onTap: () => _open(book),
            );
          },
        );
      case _AudiobookPane.history:
        if (store.history.isEmpty) {
          return const StatusPanel(
            title: '还没有听过',
            message: '打开一章之后，这里会记下章节和听到的秒数。',
            icon: Icons.history_rounded,
          );
        }
        return ListView.builder(
          itemCount: store.history.length,
          itemBuilder: (context, index) {
            final entry = store.history[index];
            return ListTile(
              key: ValueKey('audiobook-history-${entry.book.id}'),
              title: Text(entry.book.title),
              subtitle: Text(
                '${entry.chapterTitle} · ${formatAudiobookClock(entry.seconds)}',
              ),
              onTap: () => _open(entry.book, resume: true),
            );
          },
        );
      case _AudiobookPane.search:
        break;
    }
    if (_loading) return const Center(child: CircularProgressIndicator());
    if (_error != null && _books.isEmpty) {
      return StatusPanel(
        title: '没有听书结果',
        message: _error!,
        icon: Icons.headphones_rounded,
        onRetry: () =>
            _query.isEmpty && _board != null ? _loadBoard(_board!) : _run(),
      );
    }
    if (_books.isEmpty) {
      return const StatusPanel(
        title: '搜索番茄畅听',
        message: '听书榜会列在这里。也可以搜索书名，或点上面的词。',
        icon: Icons.headphones_rounded,
      );
    }
    return ListView.builder(
      itemCount: _books.length + (_hasMore ? 1 : 0),
      itemBuilder: (context, index) {
        if (index >= _books.length) {
          return TextButton(
            onPressed: _loadingMore
                ? null
                : () => _query.isEmpty && _board != null
                      ? _loadBoard(_board!, more: true)
                      : _run(null, true),
            child: Text(_loadingMore ? '加载中' : '继续'),
          );
        }
        final book = _books[index];
        return ListTile(
          key: ValueKey('audiobook-${book.id}'),
          title: Text(book.title),
          subtitle: Text(_bookLine(book)),
          onTap: () => _open(book),
        );
      },
    );
  }

  String _bookLine(FanqieBook book) =>
      [book.author, book.category].where((item) => item.isNotEmpty).join(' · ');
}

class AudiobookCatalogScreen extends StatefulWidget {
  const AudiobookCatalogScreen({
    super.key,
    required this.api,
    required this.repository,
    required this.store,
    required this.book,
    this.resumeOnOpen = false,
  });

  final FanqieAudiobookApi api;
  final AppRepository repository;
  final AudiobookStore store;
  final FanqieBook book;
  final bool resumeOnOpen;

  @override
  State<AudiobookCatalogScreen> createState() => _AudiobookCatalogScreenState();
}

class _AudiobookCatalogScreenState extends State<AudiobookCatalogScreen> {
  final List<FanqieChapter> _chapters = [];
  int _shown = fanqieChapterPageSize;
  bool _loading = true;
  bool _busy = false;
  bool _stopDownload = false;
  bool _didResume = false;
  String? _error;
  String _status = '';

  @override
  void initState() {
    super.initState();
    widget.store.addListener(_onStore);
    _load();
  }

  @override
  void dispose() {
    _stopDownload = true;
    widget.store.removeListener(_onStore);
    super.dispose();
  }

  void _onStore() {
    if (mounted) setState(() {});
  }

  Future<void> _load() async {
    setState(() {
      _loading = _chapters.isEmpty;
      _error = null;
    });
    try {
      final chapters = await widget.api.allChapters(widget.book.id);
      if (!mounted) return;
      setState(() {
        _chapters
          ..clear()
          ..addAll(chapters);
        _loading = false;
      });
      if (widget.resumeOnOpen && !_didResume) {
        _didResume = true;
        final latest = widget.store.latest(widget.book.id);
        if (latest == null) return;
        for (final chapter in _chapters) {
          if (chapter.id == latest.chapterId) {
            await _play(chapter);
            break;
          }
        }
      }
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = '$error';
      });
    }
  }

  Future<void> _play(FanqieChapter chapter) async {
    final index = _chapters.indexWhere((item) => item.id == chapter.id);
    if (index < 0 || !mounted) return;
    await Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => AudiobookPlayerScreen(
          repository: widget.repository,
          api: widget.api,
          store: widget.store,
          book: widget.book,
          chapters: List<FanqieChapter>.of(_chapters),
          index: index,
        ),
      ),
    );
  }

  Future<void> _download(
    String kind, {
    String chapterId = '',
    int fromOrder = 0,
    int toOrder = 0,
  }) async {
    if (_busy) return;
    final selected = selectAudiobookDownloads(
      chapters: _chapters,
      kind: kind,
      bookId: widget.book.id,
      store: widget.store,
      chapterId: chapterId,
      fromOrder: fromOrder,
      toOrder: toOrder,
    );
    if (selected.isEmpty) {
      _toast(_emptyDownload(kind));
      return;
    }
    setState(() {
      _busy = true;
      _stopDownload = false;
      _status = '准备下载 ${selected.length} 章';
    });
    try {
      final report = await downloadAudiobookChapters(
        media: widget.repository.audiobookMedia,
        store: widget.store,
        book: widget.book,
        chapters: selected,
        cancelled: () => _stopDownload,
        onProgress: (progress) {
          if (!mounted) return;
          setState(() {
            _status = progress.currentTitle.isEmpty
                ? ''
                : '正在下载 ${progress.completed + progress.failed + 1}/${progress.total} ${progress.currentTitle}';
          });
        },
      );
      if (!mounted) return;
      setState(() => _status = '');
      if (_stopDownload) {
        _toast('已停止下载，完成 ${report.completed} 章');
      } else {
        _toast(
          report.failed > 0
              ? '已下载 ${report.completed} 章，${report.failed} 章失败'
              : '已下载 ${report.completed} 章',
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          _busy = false;
          _status = '';
        });
      }
    }
  }

  Future<void> _downloadRange() async {
    final picked = await showDialog<({int start, int end})>(
      context: context,
      builder: (_) => const _RangeDialog(),
    );
    if (picked == null || !mounted) return;
    await _download('range', fromOrder: picked.start, toOrder: picked.end);
  }

  void _toast(String text) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  String _emptyDownload(String kind) {
    switch (kind) {
      case 'single':
        return '这一章已经在本地';
      case 'range':
        return '这个范围里没有要下载的章节';
      case 'unfinished':
        return '没有还没听完的章节';
      case 'missing':
        return '没有需要补齐的章节';
      case 'all':
        return '整本已经在本地';
    }
    return '没有要下载的章节';
  }

  @override
  Widget build(BuildContext context) {
    final store = widget.store;
    final missing = store.missingFor(widget.book.id);
    final latest = store.latest(widget.book.id);
    final shown = _shown > _chapters.length ? _chapters.length : _shown;
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.book.title),
        actions: [
          IconButton(
            tooltip: store.onShelf(widget.book.id) ? '移出书架' : '加入书架',
            onPressed: () => store.toggleShelf(widget.book),
            icon: Icon(
              store.onShelf(widget.book.id)
                  ? Icons.bookmark_rounded
                  : Icons.bookmark_border_rounded,
            ),
          ),
          PopupMenuButton<String>(
            tooltip: '下载',
            enabled: !_busy && _chapters.isNotEmpty,
            onSelected: (value) {
              switch (value) {
                case 'unfinished':
                  _download('unfinished');
                case 'range':
                  _downloadRange();
                case 'all':
                  _download('all');
                case 'missing':
                  _download('missing');
              }
            },
            itemBuilder: (_) => [
              const PopupMenuItem(value: 'unfinished', child: Text('下载未听完')),
              const PopupMenuItem(value: 'range', child: Text('下载范围')),
              const PopupMenuItem(value: 'all', child: Text('下载整本')),
              PopupMenuItem(
                value: 'missing',
                enabled: missing.isNotEmpty,
                child: const Text('补齐缺章'),
              ),
            ],
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null && _chapters.isEmpty
          ? StatusPanel(
              title: '目录没有打开',
              message: _error!,
              icon: Icons.menu_book_rounded,
              onRetry: _load,
            )
          : Column(
              children: [
                if (_status.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
                    child: Row(
                      children: [
                        const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        ),
                        const SizedBox(width: 12),
                        Expanded(child: Text(_status)),
                        TextButton(
                          onPressed: () => _stopDownload = true,
                          child: const Text('停止'),
                        ),
                      ],
                    ),
                  ),
                if (missing.isNotEmpty)
                  ListTile(
                    title: Text('${missing.length} 章没有下完'),
                    subtitle: const Text('原来的进度和书签还在，可以只补这些章'),
                    trailing: FilledButton(
                      onPressed: _busy ? null : () => _download('missing'),
                      child: const Text('补齐'),
                    ),
                  ),
                if (latest != null)
                  ListTile(
                    leading: const Icon(Icons.play_circle_rounded),
                    title: Text('继续 ${latest.chapterTitle}'),
                    subtitle: Text(formatAudiobookClock(latest.seconds)),
                    onTap: () {
                      for (final chapter in _chapters) {
                        if (chapter.id == latest.chapterId) {
                          _play(chapter);
                          return;
                        }
                      }
                      _toast('上次的章节不在目录里');
                    },
                  ),
                Expanded(
                  child: _chapters.isEmpty
                      ? const StatusPanel(
                          title: '这本书没有章节',
                          message: '上游目录是空的。',
                          icon: Icons.menu_book_rounded,
                        )
                      : ListView.builder(
                          itemCount: shown + (shown < _chapters.length ? 1 : 0),
                          itemBuilder: (context, index) {
                            if (index >= shown) {
                              return TextButton(
                                onPressed: () => setState(
                                  () => _shown += fanqieChapterPageSize,
                                ),
                                child: Text('继续加载 ${_chapters.length} 章'),
                              );
                            }
                            final chapter = _chapters[index];
                            final reason = store.missingReason(
                              widget.book.id,
                              chapter.id,
                            );
                            final heard = store.heardSeconds(
                              widget.book.id,
                              chapter.id,
                            );
                            final local = store.hasLocal(
                              widget.book.id,
                              chapter.id,
                            );
                            return ListTile(
                              key: ValueKey('audiobook-chapter-${chapter.id}'),
                              title: Text(chapter.title),
                              subtitle: Text(
                                [
                                  '第 ${chapter.order} 章',
                                  if (chapter.locked) '可能需要付费',
                                  if (local) '已离线',
                                  if (heard > 1)
                                    '听到 ${formatAudiobookClock(heard)}',
                                  ?reason,
                                ].join(' · '),
                              ),
                              onTap: () => _play(chapter),
                              trailing: IconButton(
                                tooltip: '下载这一章',
                                onPressed: _busy
                                    ? null
                                    : () => _download(
                                        'single',
                                        chapterId: chapter.id,
                                      ),
                                icon: Icon(
                                  local
                                      ? Icons.download_done_rounded
                                      : Icons.download_outlined,
                                ),
                              ),
                            );
                          },
                        ),
                ),
              ],
            ),
    );
  }
}

class AudiobookPlayerScreen extends StatefulWidget {
  const AudiobookPlayerScreen({
    super.key,
    required this.repository,
    required this.api,
    required this.store,
    required this.book,
    required this.chapters,
    required this.index,
  });

  final AppRepository repository;
  final FanqieAudiobookApi api;
  final AudiobookStore store;
  final FanqieBook book;
  final List<FanqieChapter> chapters;
  final int index;

  @override
  State<AudiobookPlayerScreen> createState() => _AudiobookPlayerScreenState();
}

class _AudiobookPlayerScreenState extends State<AudiobookPlayerScreen> {
  final _player = Player();
  late final StreamSubscription<Duration> _positionSub;
  late final StreamSubscription<bool> _playingSub;
  late final StreamSubscription<bool> _completedSub;
  final _textScroll = ScrollController();
  List<GlobalKey> _lineKeys = const [];
  late int _index = widget.chapters.isEmpty
      ? 0
      : widget.index.clamp(0, widget.chapters.length - 1).toInt();
  Timer? _clock;
  Timer? _sleepTimer;
  AudiobookSleep _sleep = AudiobookSleep.off;
  bool _loading = true;
  bool _ready = false;
  bool _playing = false;
  bool _dragging = false;
  bool _caughtUp = true;
  bool _local = false;
  String? _error;
  double _speed = 1;
  double _position = 0;
  double _duration = 0;
  double _drag = 0;
  double _resumeFloor = 0;
  int _generation = 0;
  bool _opening = false;
  bool _handledEnd = false;
  bool _heardProgress = false;
  bool _asleep = false;
  bool _reading = true;
  bool _following = true;
  bool _timed = false;
  Object? _timedText;
  bool _textLoading = false;
  int _line = -1;
  int _shownLine = -1;
  String? _textError;
  List<String> _texts = const [];
  List<AudiobookLine> _lines = const [];

  @override
  void initState() {
    super.initState();
    _positionSub = _player.stream.position.listen(_notePlayback);
    _playingSub = _player.stream.playing.listen((playing) {
      if (!mounted) return;
      setState(() => _playing = playing);
      if (!playing) unawaited(_persist());
    });
    _completedSub = _player.stream.completed.listen((done) {
      if (done && _heardProgress) _finishChapter();
    });
    _clock = Timer.periodic(const Duration(seconds: 5), (_) {
      if (_playing) unawaited(_persist());
    });
    if (widget.chapters.isNotEmpty) _open();
  }

  @override
  void dispose() {
    _clock?.cancel();
    _sleepTimer?.cancel();
    unawaited(_positionSub.cancel());
    unawaited(_playingSub.cancel());
    unawaited(_completedSub.cancel());
    if (_ready && widget.chapters.isNotEmpty) {
      final seconds = _secondsToSave();
      final duration = _player.state.duration.inMilliseconds / 1000;
      unawaited(
        widget.store.remember(
          widget.book,
          _chapter,
          seconds,
          duration: duration,
        ),
      );
    }
    _textScroll.dispose();
    _player.dispose();
    super.dispose();
  }

  FanqieChapter get _chapter => widget.chapters[_index];

  double _secondsToSave() {
    var seconds = _player.state.position.inMilliseconds / 1000;
    if (!_caughtUp && seconds + 1 < _resumeFloor) seconds = _resumeFloor;
    if (seconds < 0) return 0;
    return seconds;
  }

  Future<void> _persist() async {
    if (!_ready || widget.chapters.isEmpty) return;
    final duration = _player.state.duration.inMilliseconds / 1000;
    await widget.store.remember(
      widget.book,
      _chapter,
      _secondsToSave(),
      duration: duration,
    );
  }

  void _notePlayback(Duration position) {
    final seconds = position.inMilliseconds / 1000;
    final duration = _player.state.duration.inMilliseconds / 1000;
    if (_resumeFloor <= 1 || seconds + 2 >= _resumeFloor) _caughtUp = true;
    if (duration > 1 && seconds + 1.5 < duration) _heardProgress = true;
    if (_heardProgress && duration > 1 && seconds >= duration - 0.35) {
      _finishChapter();
    }
    _followLine(seconds);
    if (!mounted || _dragging) return;
    setState(() {
      _position = seconds;
      if (duration > 0) _duration = duration;
    });
    if ((_texts.isNotEmpty || _timedText != null) &&
        (_lines.isEmpty || _lines.last.end != _duration)) {
      _refreshLines();
    }
  }

  void _finishChapter() {
    if (_handledEnd || !_ready || _opening || _asleep) return;
    _handledEnd = true;
    if (_sleep == AudiobookSleep.chapter) {
      _sleep = AudiobookSleep.off;
      unawaited(_player.pause());
      if (mounted) setState(() {});
      return;
    }
    if (_index + 1 >= widget.chapters.length) return;
    unawaited(_step(1));
  }

  void _refreshLines() {
    final timed = _timedText == null
        ? const <AudiobookLine>[]
        : audiobookTimedLines(_timedText, _duration);
    _lines = timed.isNotEmpty ? timed : paceAudiobookLines(_texts, _duration);
    if (_lineKeys.length != _lines.length) {
      _lineKeys = [for (var i = 0; i < _lines.length; i++) GlobalKey()];
    }
    _line = audiobookLineAt(_lines, _position);
  }

  void _followLine(double seconds, {bool force = false}) {
    final next = audiobookLineAt(_lines, seconds);
    if (next == _line && !force) return;
    _line = next;
    if (!_reading || !_following || next < 0) return;
    if (next == _shownLine && !force) return;
    _shownLine = next;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (next >= _lineKeys.length) return;
      final target = _lineKeys[next].currentContext;
      if (target == null || !mounted) return;
      Scrollable.ensureVisible(
        target,
        alignment: 0.35,
        duration: const Duration(milliseconds: 220),
      );
    });
  }

  Future<void> _loadText() async {
    final itemId = _chapter.id;
    final generation = _generation;
    setState(() {
      _textLoading = true;
      _textError = null;
      _texts = const [];
      _timedText = null;
      _timed = false;
      _lines = const [];
      _line = -1;
      _shownLine = -1;
    });
    try {
      // 先取带逐句时间轴的听书正文；失败再退回网页正文，按字数比例对齐。
      Object? timedText;
      try {
        timedText = await widget.repository.audiobookText(itemId);
      } catch (_) {
        timedText = null;
      }
      if (!mounted || generation != _generation) return;
      var texts = const <String>[];
      if (audiobookTimedLines(timedText, 1).isEmpty) {
        timedText = null;
        texts = await widget.api.chapterText(itemId);
      }
      if (!mounted || generation != _generation) return;
      setState(() {
        _timedText = timedText;
        _timed = timedText is Map && timedText['timed'] == true;
        _texts = texts;
        _textLoading = false;
        _textError = timedText == null && texts.isEmpty ? '这一章暂时没有原文' : null;
        _refreshLines();
      });
      _followLine(_position, force: true);
    } catch (error) {
      if (!mounted || generation != _generation) return;
      setState(() {
        _textLoading = false;
        _textError = '$error';
      });
    }
  }

  Future<void> _open({double? at}) async {
    final generation = ++_generation;
    _opening = true;
    _handledEnd = true;
    _heardProgress = false;
    final chapter = _chapter;
    final localFile = widget.store.hasLocal(widget.book.id, chapter.id);
    final resume = at ?? widget.store.heardSeconds(widget.book.id, chapter.id);
    _ready = false;
    _resumeFloor = resume < 0 ? 0 : resume;
    _caughtUp = _resumeFloor <= 1;
    _local = localFile;
    if (mounted) {
      setState(() {
        _loading = true;
        _error = null;
        _position = _resumeFloor;
        _texts = const [];
        _timedText = null;
        _lines = const [];
        _line = -1;
        _textError = null;
      });
    }
    try {
      Map<String, dynamic>? remote;
      if (!localFile) {
        remote = await widget.repository.audiobookMedia(
          widget.book.id,
          chapter.id,
        );
        if (generation != _generation || !mounted) return;
        final duration = (remote['duration'] as num?)?.toDouble() ?? 0;
        if (duration > 0) {
          await widget.store.noteDuration(widget.book.id, chapter.id, duration);
        }
      }
      if (generation != _generation || !mounted) return;
      final target = audiobookOpenTarget(
        store: widget.store,
        bookId: widget.book.id,
        chapterId: chapter.id,
        remote: remote,
      );
      if (target.url.isEmpty) throw AppFailure('这一章暂时没有音频地址');
      final platform = _player.platform;
      if (platform is NativePlayer) {
        if (Platform.isAndroid) {
          await platform.setProperty('ao', 'audiotrack,opensles');
        }
        await platform.setProperty(
          'demuxer-lavf-o',
          [
            'strict=experimental',
            target.local
                ? 'protocol_whitelist=[file,crypto,data]'
                : 'protocol_whitelist=[http,https,tcp,tls,crypto,data,file]',
            if (target.key.isNotEmpty) 'decryption_key=${target.key}',
          ].join(','),
        );
        if (!target.local) await platform.setProperty('network-timeout', '20');
      }
      if (generation != _generation || !mounted) return;
      await _player.setRate(_speed);
      await _player.open(
        Media(
          target.url,
          httpHeaders: target.headers,
          start: _resumeFloor > 0.5
              ? Duration(milliseconds: (_resumeFloor * 1000).round())
              : null,
        ),
      );
      if (generation != _generation || !mounted) return;
      _ready = true;
      _opening = false;
      _handledEnd = false;
      _local = target.local;
      setState(() {
        _loading = false;
        _duration = _player.state.duration.inMilliseconds / 1000;
      });
      _refreshLines();
      unawaited(_loadText());
    } catch (error) {
      if (generation != _generation || !mounted) return;
      _ready = false;
      _opening = false;
      final message = '$error';
      if (localFile) {
        await widget.store.markMissing(widget.book.id, chapter.id, message);
      }
      if (!mounted || generation != _generation) return;
      setState(() {
        _loading = false;
        _error = message;
      });
    }
  }

  Future<void> _seekBy(int seconds) async {
    final duration = _player.state.duration.inMilliseconds / 1000;
    final target = seekAudiobookSeconds(_secondsToSave(), seconds, duration);
    _caughtUp = true;
    await _player.seek(Duration(milliseconds: (target * 1000).round()));
    if (mounted) setState(() => _position = target);
    await _persist();
  }

  Future<void> _step(int delta) async {
    final next = _index + delta;
    if (next < 0 || next >= widget.chapters.length) return;
    await _persist();
    if (!mounted) return;
    setState(() => _index = next);
    await _open();
  }

  void _armSleep(AudiobookSleep sleep) {
    _sleepTimer?.cancel();
    _sleepTimer = null;
    _asleep = false;
    setState(() => _sleep = sleep);
    if (sleep == AudiobookSleep.off || sleep == AudiobookSleep.chapter) return;
    _sleepTimer = Timer(Duration(minutes: sleep.minutes), () async {
      if (!mounted) return;
      _asleep = true;
      setState(() => _sleep = AudiobookSleep.off);
      await _player.pause();
    });
  }

  Future<void> _jumpLine(AudiobookLine line, [double? at]) async {
    final target = at ?? line.start;
    _caughtUp = true;
    _handledEnd = false;
    _following = true;
    await _player.seek(Duration(milliseconds: (target * 1000).round()));
    if (mounted) setState(() => _position = target);
    _followLine(target, force: true);
    await _persist();
  }

  /// 双击某段：按双击处的字符落在哪一句，跳到那一句的时间。
  void _doubleTapLine(int index, Offset? local) {
    if (index < 0 || index >= _lines.length) return;
    final line = _lines[index];
    var at = line.start;
    final render = _lineKeys[index].currentContext?.findRenderObject();
    if (local != null && render is RenderBox) {
      RenderParagraph? paragraph;
      void visit(RenderObject child) {
        if (paragraph != null) return;
        if (child is RenderParagraph) {
          paragraph = child;
          return;
        }
        child.visitChildren(visit);
      }

      render.visitChildren(visit);
      final found = paragraph;
      if (found != null) {
        final global = render.localToGlobal(local);
        final offset = found
            .getPositionForOffset(found.globalToLocal(global))
            .offset;
        at = audiobookSeekForOffset(line, offset);
      }
    }
    unawaited(_jumpLine(line, at));
  }

  void _resumeFollow() {
    setState(() => _following = true);
    _followLine(_position, force: true);
  }

  Future<void> _addBookmark() async {
    await widget.store.addBookmark(widget.book, _chapter, _secondsToSave());
    if (mounted) _toast('已加书签');
  }

  Future<void> _showBookmarks() async {
    final marks = widget.store.bookmarks(widget.book.id);
    if (marks.isEmpty) {
      _toast('还没有书签');
      return;
    }
    final picked = await showModalBottomSheet<AudiobookBookmark>(
      context: context,
      builder: (context) => SafeArea(
        child: ListView(
          children: [
            for (final mark in marks)
              ListTile(
                title: Text(mark.chapterTitle),
                subtitle: Text(formatAudiobookClock(mark.seconds)),
                onTap: () => Navigator.pop(context, mark),
              ),
          ],
        ),
      ),
    );
    if (picked == null || !mounted) return;
    final index = widget.chapters.indexWhere(
      (item) => item.id == picked.chapterId,
    );
    if (index < 0) {
      _toast('这一章不在当前目录里');
      return;
    }
    if (index == _index) {
      _caughtUp = true;
      await _player.seek(
        Duration(milliseconds: (picked.seconds * 1000).round()),
      );
      if (mounted) setState(() => _position = picked.seconds);
      await _persist();
      return;
    }
    await _persist();
    if (!mounted) return;
    setState(() => _index = index);
    await _open(at: picked.seconds);
  }

  void _toast(String text) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  @override
  Widget build(BuildContext context) {
    if (widget.chapters.isEmpty) {
      return Scaffold(
        appBar: AppBar(title: Text(widget.book.title)),
        body: const StatusPanel(
          title: '没有可以播放的章节',
          message: '先回到目录再选一章。',
          icon: Icons.headphones_rounded,
        ),
      );
    }
    final chapter = _chapter;
    final max = _duration > 1 ? _duration : 1.0;
    final value = (_dragging ? _drag : _position).clamp(0.0, max).toDouble();
    final marks = widget.store.bookmarks(widget.book.id);
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.book.title),
        actions: [
          IconButton(
            tooltip: _reading ? '收起原文' : '原文',
            onPressed: () {
              setState(() => _reading = !_reading);
              if (_reading && _texts.isEmpty && !_textLoading) {
                unawaited(_loadText());
              }
            },
            icon: Icon(
              _reading
                  ? Icons.close_fullscreen_rounded
                  : Icons.menu_book_rounded,
            ),
          ),
          IconButton(
            tooltip: '加书签',
            onPressed: _addBookmark,
            icon: const Icon(Icons.bookmark_add_outlined),
          ),
          IconButton(
            tooltip: '书签',
            onPressed: _showBookmarks,
            icon: Badge(
              isLabelVisible: marks.isNotEmpty,
              label: Text('${marks.length}'),
              child: const Icon(Icons.bookmarks_outlined),
            ),
          ),
        ],
      ),
      body: _reading
          ? _manuscript(chapter)
          : Padding(
              padding: const EdgeInsets.all(20),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    chapter.title,
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    _loading
                        ? '正在打开'
                        : _error != null
                        ? _error!
                        : _local
                        ? '离线播放'
                        : '在线播放',
                  ),
                  if (_loading) const LinearProgressIndicator(),
                  if (_error != null)
                    Align(
                      alignment: Alignment.centerLeft,
                      child: TextButton(
                        onPressed: () => _open(),
                        child: const Text('重试'),
                      ),
                    ),
                  const Spacer(),
                  Row(
                    children: [
                      Text(formatAudiobookClock(_dragging ? _drag : _position)),
                      Expanded(
                        child: Slider(
                          value: value,
                          max: max,
                          onChanged: _duration > 1
                              ? (next) => setState(() {
                                  _dragging = true;
                                  _drag = next;
                                })
                              : null,
                          onChangeEnd: _duration > 1
                              ? (next) async {
                                  setState(() {
                                    _dragging = false;
                                    _position = next;
                                  });
                                  _caughtUp = true;
                                  await _player.seek(
                                    Duration(
                                      milliseconds: (next * 1000).round(),
                                    ),
                                  );
                                  await _persist();
                                }
                              : null,
                        ),
                      ),
                      Text(formatAudiobookClock(_duration)),
                    ],
                  ),
                  const SizedBox(height: 8),
                  Wrap(
                    spacing: 8,
                    children: [
                      for (final speed in const [0.75, 1.0, 1.25, 1.5, 2.0])
                        ChoiceChip(
                          label: Text(speed == 1 ? '1x' : '${speed}x'),
                          selected: _speed == speed,
                          onSelected: (_) async {
                            setState(() => _speed = speed);
                            await _player.setRate(speed);
                          },
                        ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  Wrap(
                    spacing: 8,
                    children: [
                      for (final sleep in AudiobookSleep.values)
                        ChoiceChip(
                          label: Text(sleep.label),
                          selected: _sleep == sleep,
                          onSelected: (_) => _armSleep(sleep),
                        ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                    children: [
                      IconButton(
                        tooltip: '上一章',
                        onPressed: _index > 0 ? () => _step(-1) : null,
                        icon: const Icon(Icons.skip_previous_rounded),
                      ),
                      IconButton(
                        tooltip: '后退 15 秒',
                        onPressed: () => _seekBy(-15),
                        icon: const Icon(CupertinoIcons.gobackward_15),
                      ),
                      IconButton(
                        tooltip: _playing ? '暂停' : '播放',
                        onPressed: () => _player.playOrPause(),
                        icon: Icon(
                          _playing
                              ? Icons.pause_rounded
                              : Icons.play_arrow_rounded,
                        ),
                      ),
                      IconButton(
                        tooltip: '前进 15 秒',
                        onPressed: () => _seekBy(15),
                        icon: const Icon(CupertinoIcons.goforward_15),
                      ),
                      IconButton(
                        tooltip: '下一章',
                        onPressed: _index + 1 < widget.chapters.length
                            ? () => _step(1)
                            : null,
                        icon: const Icon(Icons.skip_next_rounded),
                      ),
                    ],
                  ),
                ],
              ),
            ),
    );
  }

  Widget _manuscript(FanqieChapter chapter) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Expanded(
          child: _textLoading
              ? const Center(child: CircularProgressIndicator())
              : _lines.isEmpty
              ? StatusPanel(
                  title: '没有原文',
                  message: _textError ?? '这一章暂时没有原文',
                  icon: Icons.menu_book_rounded,
                  onRetry: () => unawaited(_loadText()),
                )
              : Stack(
                  children: [
                    NotificationListener<UserScrollNotification>(
                      onNotification: (notification) {
                        if (notification.direction != ScrollDirection.idle &&
                            _following) {
                          setState(() => _following = false);
                        }
                        return false;
                      },
                      child: SingleChildScrollView(
                        controller: _textScroll,
                        padding: const EdgeInsets.fromLTRB(20, 8, 20, 72),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            for (var index = 0; index < _lines.length; index++)
                              _manuscriptLine(theme, index),
                          ],
                        ),
                      ),
                    ),
                    if (!_following)
                      Positioned(
                        right: 16,
                        bottom: 12,
                        child: FloatingActionButton.extended(
                          key: const ValueKey('audiobook-follow'),
                          heroTag: null,
                          onPressed: _resumeFollow,
                          icon: const Icon(Icons.my_location_rounded),
                          label: const Text('跟随朗读'),
                        ),
                      ),
                  ],
                ),
        ),
        _miniControls(theme),
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 16),
          child: Text(
            _timed
                ? '${chapter.title} · 双击句子跳到这里 · 逐句时间轴'
                : '${chapter.title} · 双击段落跳到这里 · 按字数估算位置',
            style: theme.textTheme.bodySmall,
          ),
        ),
      ],
    );
  }

  Widget _manuscriptLine(ThemeData theme, int index) {
    final line = _lines[index];
    final current = index == _line;
    final style = theme.textTheme.bodyLarge?.copyWith(height: 1.65);
    final segment = current ? audiobookSegmentAt(line, _position) : -1;
    Offset? tapAt;
    return Padding(
      key: _lineKeys[index],
      padding: const EdgeInsets.only(bottom: 8),
      child: Material(
        color: current
            ? theme.colorScheme.primaryContainer
            : Colors.transparent,
        borderRadius: BorderRadius.circular(10),
        child: GestureDetector(
          key: ValueKey('audiobook-line-$index'),
          behavior: HitTestBehavior.opaque,
          onDoubleTapDown: (details) => tapAt = details.localPosition,
          onDoubleTap: () => _doubleTapLine(index, tapAt),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            child: line.segments.isEmpty
                ? Text(
                    line.text,
                    style: style?.copyWith(
                      fontWeight: current ? FontWeight.w600 : null,
                    ),
                  )
                : Text.rich(
                    TextSpan(
                      children: [
                        for (var i = 0; i < line.segments.length; i++)
                          TextSpan(
                            text: line.segments[i].text,
                            style: i == segment
                                ? TextStyle(
                                    fontWeight: FontWeight.w700,
                                    color: theme.colorScheme.primary,
                                  )
                                : null,
                          ),
                      ],
                    ),
                    style: style,
                  ),
          ),
        ),
      ),
    );
  }

  Widget _miniControls(ThemeData theme) {
    final max = _duration > 0 ? _duration : 1.0;
    final value = (_dragging ? _drag : _position).clamp(0, max).toDouble();
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12),
      child: Row(
        children: [
          IconButton(
            tooltip: '后退 15 秒',
            onPressed: () => _seekBy(-15),
            icon: const Icon(CupertinoIcons.gobackward_15),
          ),
          IconButton(
            key: const ValueKey('audiobook-mini-play'),
            tooltip: _playing ? '暂停' : '播放',
            onPressed: () => _player.playOrPause(),
            icon: Icon(
              _playing ? Icons.pause_rounded : Icons.play_arrow_rounded,
            ),
          ),
          IconButton(
            tooltip: '前进 15 秒',
            onPressed: () => _seekBy(15),
            icon: const Icon(CupertinoIcons.goforward_15),
          ),
          Expanded(
            child: Slider(
              value: value,
              max: max,
              onChanged: _duration > 1
                  ? (next) => setState(() {
                      _dragging = true;
                      _drag = next;
                    })
                  : null,
              onChangeEnd: _duration > 1
                  ? (next) async {
                      setState(() {
                        _dragging = false;
                        _position = next;
                        _following = true;
                      });
                      _caughtUp = true;
                      await _player.seek(
                        Duration(milliseconds: (next * 1000).round()),
                      );
                      _followLine(next, force: true);
                      await _persist();
                    }
                  : null,
            ),
          ),
          Text(
            '${formatAudiobookClock(value)} / ${formatAudiobookClock(_duration)}',
            style: theme.textTheme.bodySmall,
          ),
        ],
      ),
    );
  }
}

class _RangeDialog extends StatefulWidget {
  const _RangeDialog();

  @override
  State<_RangeDialog> createState() => _RangeDialogState();
}

class _RangeDialogState extends State<_RangeDialog> {
  final _from = TextEditingController();
  final _to = TextEditingController();

  @override
  void dispose() {
    _from.dispose();
    _to.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('下载范围'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TextField(
            controller: _from,
            keyboardType: TextInputType.number,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            decoration: const InputDecoration(labelText: '起始章节号'),
          ),
          TextField(
            controller: _to,
            keyboardType: TextInputType.number,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            decoration: const InputDecoration(labelText: '结束章节号'),
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('取消'),
        ),
        FilledButton(
          onPressed: () {
            final start = int.tryParse(_from.text.trim());
            final end = int.tryParse(_to.text.trim());
            if (start == null || end == null || start < 1 || end < 1) return;
            Navigator.pop(context, (start: start, end: end));
          },
          child: const Text('下载'),
        ),
      ],
    );
  }
}
