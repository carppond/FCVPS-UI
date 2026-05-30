# Android 编译、打包与安装

> Android 不像 iOS：APK **谁都能直接装**，不需要安装者自己重签。Release 提供一个 APK，用户下载后允许"未知来源"即可安装。无小组件/App Group 的付费账号限制。

## 一、安装（普通用户）

1. 从 GitHub Release 下载 `shiguang-vps.apk`。
2. 手机 → 设置 → 允许该来源安装应用（"安装未知应用"）。
3. 点 APK 安装、打开即可。

> Android 没有 iOS 那种 7 天签名过期；装上一直能用。

---

## 二、编译环境（开发者）

只在需要自己出包/改代码时用。

| 工具 | 版本 | 说明 |
|------|------|------|
| JDK | **17**（必须，不是 11/21） | `brew install openjdk@17` 后设 `JAVA_HOME`；Expo SDK 56 / AGP 8 要求 17 |
| Android SDK | platform-tools + build-tools 34+ + platform android-34 | 装 Android Studio 最省事，或用 cmdline-tools + `sdkmanager` |
| 环境变量 | `ANDROID_HOME=~/Library/Android/sdk`（macOS） | 并把 `platform-tools`、`cmdline-tools/latest/bin` 加进 `PATH` |
| Node / pnpm | 20.x / 9+ | |

```bash
# 例（macOS，按实际路径调整）
export JAVA_HOME="$(/usr/libexec/java_home -v 17)"
export ANDROID_HOME="$HOME/Library/Android/sdk"
export PATH="$PATH:$ANDROID_HOME/platform-tools:$ANDROID_HOME/cmdline-tools/latest/bin"
sdkmanager "platform-tools" "platforms;android-34" "build-tools;34.0.0"   # 首次装组件 + 接受 license
```

---

## 三、本地编译 APK

```bash
cd mobile
pnpm install

# 1. 生成 android 原生工程
npx expo prebuild -p android --clean

# 2. 编 Release APK
cd android
./gradlew assembleRelease         # 产物：app/build/outputs/apk/release/app-release.apk
# 调试包(必出，不需配签名):./gradlew assembleDebug → app-debug.apk
```

- 产物路径：`mobile/android/app/build/outputs/apk/release/app-release.apk`
- Expo 模板的 release 默认用 debug 签名 keystore，所以 `assembleRelease` 直接能产出**可安装**的 APK。要正式发布签名见第五节。
- 改名上传 Release：`cp app/build/outputs/apk/release/app-release.apk shiguang-vps.apk`

> 小组件是 iOS 专属（expo-widgets），Android 不涉及，**不需要 `EXPO_WIDGET`**，一个 APK 即可。

---

## 四、用 EAS 云端构建（无需本地 Android 环境，最省事）

不想装 JDK/SDK 就用 Expo 云构建：

```bash
cd mobile
npm i -g eas-cli
eas login                                   # 登录你的 Expo 账号
eas build -p android --profile preview      # 云端出 APK(preview profile 已配为 APK)
```
构建完成后 EAS 给一个下载链接，下下来就是可安装 APK。仓库已内置 `mobile/eas.json` 的 `preview` profile（`buildType: apk`）。

> 免费额度有限；开源项目把代码传到 Expo 云没问题（代码本就公开）。

---

## 五、正式签名（上架 / 长期发布，可选）

自建发布用自己的 keystore：

```bash
keytool -genkey -v -keystore shiguang.keystore -alias shiguang \
  -keyalg RSA -keysize 2048 -validity 10000
```
然后在 `android/app/build.gradle` 的 `signingConfigs.release` 指向该 keystore（或用 `eas credentials` 让 EAS 托管签名）。`keystore` / 密码**不要提交进仓库**。

---

## 六、常见问题

- `Unsupported class file major version` / Gradle 报 Java 版本：JDK 不是 17。`export JAVA_HOME="$(/usr/libexec/java_home -v 17)"`。
- `SDK location not found`：没设 `ANDROID_HOME`，或 `android/local.properties` 缺 `sdk.dir=`。
- `Failed to install ... INSTALL_FAILED_UPDATE_INCOMPATIBLE`：之前装过签名不同的同包名 App，先卸载再装。
- 安装被拦：开启"允许此来源安装未知应用"。

> iOS 打包/自签见 [ios-build.md](./ios-build.md)。
