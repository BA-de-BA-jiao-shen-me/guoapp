# 短剧集合

短剧集合是独立运行的多端短剧应用。目录、搜索、下载和播放都在本机完成，不依赖自建服务。当前源码版本是 **0.2.132+2138**。

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
