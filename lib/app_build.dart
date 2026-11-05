const allSourcesEnabled = bool.fromEnvironment('ALL_SOURCES');
const appName = '短剧集合';
const appSlug = allSourcesEnabled ? 'zhenguojian' : 'lvguojian';

String appEditionName(bool fullMode) => appName;
