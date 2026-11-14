# 短剧集合

短剧集合是独立运行的多端短剧应用。目录、搜索、下载和播放都在本机完成，不依赖自建服务。当前源码版本是 **0.2.136+2142**。

支持 Android、Windows 和 iOS。默认构建只包含绿色站源；需要全部站源时，给构建脚本加上 `--all-sources`。两套构建的产物文件名不同，不要混用同一份原生库。

历史版本说明在 [CHANGELOG.md](CHANGELOG.md)，按版本折叠。

## 环境

- Flutter 3.47.4，Dart 3.12 或更新版本
- Go 1.26.0 或更新版本
- Python 3.10 或更新版本
- Android：JDK 17、Android SDK、NDK `28.2.13676358`，并设置 `ANDROID_HOME`
- Windows：Visual Studio 的「使用 C++ 的桌面开发」，以及 MinGW-w64 x64
- iOS：macOS、完整 Xcode、CocoaPods

把 Flutter、Go、Python 放进 `PATH`。构建脚本只给本次子进程设置 `GOPROXY=https://goproxy.cn,direct` 和 `GOSUMDB=off`，不改本机全局配置。同名环境变量可以覆盖。

## 构建

在仓库根目录执行。国内网络可以加 `--cn-mirrors`。

Android：

```powershell
python scripts/build_android.py --abi arm64-v8a --cn-mirrors
python scripts/build_android.py --all-sources --abi arm64-v8a --cn-mirrors
```

`--abi` 还可以是 `armeabi-v7a` 或 `x86_64`。可以重复传入，一次打出多个架构。省略时按脚本默认的分包方式构建。

Windows：

```powershell
python scripts/build_windows.py --cn-mirrors
python scripts/build_windows.py --all-sources --cn-mirrors
```

也可以用 `.\scripts\build_windows.ps1`，对应开关是 `-ChinaMirrors` 和 `-AllSources`。

iOS 只能在 macOS 上构建：

```sh
python3 scripts/build_ios.py
python3 scripts/build_ios.py --all-sources
python3 scripts/build_ios.py --export-options /path/to/ExportOptions.plist
```

不带导出配置时得到未签名应用。签名 IPA 使用你自己的描述文件和 ExportOptions。

产物在 `dist/android`、`dist/windows`、`dist/ios`。默认版 Windows 程序是 `lvguojian.exe`，解压后保留同目录的 DLL 和 `data`。安装包架构和手机不一致时，系统会报 `INSTALL_FAILED_NO_MATCHING_ABIS`。

## 签名

正式 Android 包沿用同一把密钥，并递增版本号。密钥放在仓库外面，在 `android/key.properties` 里指向它。这个文件不要提交。

```properties
storeFile=C:/path/to/release.jks
storePassword=你的密码
keyAlias=release
keyPassword=你的密码
```

没有这份文件时，发布包使用调试签名。不同电脑打出的调试包不能互相覆盖安装。

```powershell
keytool -genkeypair -v -keystore release.jks -storetype JKS -alias release -keyalg RSA -keysize 2048 -validity 10000
```

## GitHub Actions

推送到 `main` 或 `master`、打 `v*` 标签、打开 Pull Request，或手动运行 **Build app packages**，会先做检查，再构建 Android 和 Windows。Android 正式签名读取这些仓库 Secrets：

| Secret | 内容 |
| --- | --- |
| `ANDROID_KEYSTORE_BASE64` | JKS 文件的 Base64 |
| `ANDROID_KEYSTORE_PASSWORD` | 密钥库密码 |
| `ANDROID_KEY_ALIAS` | 密钥别名 |
| `ANDROID_KEY_PASSWORD` | 密钥密码 |

未配置时上传的是预览包。产物保留 14 天，不会自动创建 Release。

## 本地调试

先编译对应平台的原生核心，再启动 Flutter。

```powershell
python scripts/build_native.py --platform android --abi arm64-v8a
flutter pub get --enforce-lockfile
flutter run
```

Windows 把 `--platform` 换成 `windows`，然后执行 `flutter run -d windows`。全部站源构建给原生构建加上 `--all-sources`，给 Flutter 加上 `--dart-define=ALL_SOURCES=true`。换回默认构建时要重新构建默认核心，不要和全部站源构建的原生库混用。

## 红果远程配置

红果取流方式由 `config/hongguo-remote-config.json` 控制，应用不用重新打包就能改顺序、停用失效的方式、调整探测设备数和接口路径。原生核心内置同一份文件（`native/core/hongguo_remote_config.json`，测试会检查两份一致），启动后在后台依次尝试 raw.githubusercontent.com、jsDelivr（cdn / gcore / fastly），都失败时再试第三方加速代理 gh-proxy.com、ghfast.top；第一份通过校验的配置生效，每次请求最多等 8 秒，不阻塞播放。通过校验的配置写入数据目录的 `hongguo_remote_config.json`，下次启动直接使用。

修改步骤：改 `config/hongguo-remote-config.json`，**把 `revision` 调大**（比当前小的配置不会生效），再推送到 main。raw.githubusercontent.com 约 5 分钟内更新；jsDelivr 对分支最长缓存约 12 小时，可以打开 `https://purge.jsdelivr.net/gh/BA-de-BA-jiao-shen-me/guoapp@main/config/hongguo-remote-config.json` 刷新。客户端按 `refreshHours` 定期重新拉取。

| 字段 | 说明与范围 |
| --- | --- |
| `schema` | 固定为 1 |
| `revision` | `YYYYMMDDNN` 格式（例如 2026111501），日期不能晚于两天后；只接受不小于当前的值 |
| `minCoreLevel` | 需要的核心能力等级；高于应用所带核心时整份配置不生效（当前核心为 1） |
| `strategies` | 取流方式及顺序：`multi_unsigned`、`video_model_signed`、`multi_signed`、`multi_unsigned_1`、`multi_signed_1`；不认识的名称跳过，至少要有一个认识的 |
| `probeDevices` | 已验证设备失效后最多注册几台新设备，1–20 |
| `reuseDevice` | 是否复用并保存验证可用的设备 |
| `deviceTTLHours` | 保存的设备多久后作废，0–168；0 表示不跨重启保存 |
| `refreshHours` | 多久重新拉取一次配置，1–168 |
| `paths.videoModel` / `paths.multiVideoModel` | 取流接口路径，必须是 `/novel/player/…/` 形式 |
| `playback.versionCode` / `versionName` / `userAgent` | 无签名取流使用的版本参数和 User-Agent |

配置里不放任何密钥。不认识的字段会被忽略，任一字段不合法时整份配置丢弃，继续用缓存或内置配置。调试时可用环境变量 `DUANJU_HONGGUO_CONFIG_URLS`（逗号分隔，https 或本机 http）替换拉取地址。验证可用的设备另存为数据目录的 `hongguo_playback_device.json`。
