# iOS 编译、打包与自签安装

> iOS 不像 Android 能直接发一个"谁都能装"的安装包。任何 IPA 要装上 iPhone，**必须用安装者自己的 Apple 证书重新签名**（Sideloadly / AltStore），或走 TestFlight / App Store。本仓库 Release 提供 **未签名 IPA**，由你用自己的 Apple ID 重签后安装。

## 一、两个 IPA 包的区别与签名要求

| 包 | 内含 | 需要的证书 / 账号 | 谁能装 |
|----|------|------------------|--------|
| **基础版** `shiguang-vps-base.ipa` | 订阅 / 探针 / 流量 / VPS 资产 / SSH 等全部功能，**不含桌面小组件** | **任意 Apple ID 即可**：免费的「Apple Development（个人团队）」证书就能重签 | 所有人（免费账号 7 天 / 付费 1 年） |
| **完整版** `shiguang-vps-widget.ipa` | 在基础版之上**追加 iOS 桌面小组件**（本月流量） | **必须付费 Apple Developer 账号**：小组件用到 **App Group**（`group.com.shiguang.vps`），免费证书无法签发该能力 | 仅付费账号；**且 App Group 与团队绑定，重签别人的包通常不可用 —— 完整版建议直接源码自建（见第四节）** |

**一句话**：
- 想要除小组件外的全部功能 → 下 **基础版**，免费账号重签即可。
- 想要小组件 → **用你自己的付费账号源码编译**（不要去重签完整版 IPA，App Group 是团队私有的，重签不会生效）。

> 说明：iOS 推送在本项目默认关闭（`enablePushNotifications:false`），所以基础版不涉及推送证书。SSH 终端功能需要**真机** + Dev Build（模拟器的 SSH 原生库不可用）。

---

## 二、编译环境要求

仅 **macOS** 能编译 iOS（Xcode 限 macOS）。本仓库验证过的环境：

| 工具 | 版本 | 说明 |
|------|------|------|
| macOS | 13+ | Apple Silicon / Intel 均可 |
| Xcode | 15+（含 26.x） | App Store 安装；首次需 `xcode-select --install` 装 Command Line Tools，并 `sudo xcodebuild -license accept` |
| Node | 20.x | |
| pnpm | 9+ | `corepack enable` 或 `npm i -g pnpm` |
| CocoaPods | 1.13+ | `sudo gem install cocoapods`（prebuild 会自动 `pod install`） |
| Apple 账号 | 免费 ID（基础版）/ 付费开发者（完整版） | Xcode → Settings → Accounts 登录 |

```bash
git clone https://github.com/carppond/FCVPS-UI.git
cd FCVPS-UI/mobile
pnpm install
```

---

## 三、编译并打包出 IPA（供用户自签）

下面产出的是 **未签名 IPA**，安装者再用自己的证书重签。

### 基础版（无小组件，任何账号可重签）

```bash
cd mobile

# 1. 生成原生工程（不要带 EXPO_WIDGET，则不含小组件 / App Group / 推送）
npx expo prebuild -p ios --clean

# 2. 编译未签名 Release .app（Xcode scheme = VPS）
xcodebuild -workspace ios/VPS.xcworkspace -scheme VPS \
  -configuration Release -sdk iphoneos \
  -derivedDataPath ios/build \
  CODE_SIGNING_ALLOWED=NO CODE_SIGNING_REQUIRED=NO

# 3. 打包成 IPA（约定俗成：.app 放进 Payload/ 再 zip）
cd ios/build/Build/Products/Release-iphoneos
mkdir -p Payload && cp -R VPS.app Payload/
zip -r ../../../../../shiguang-vps-base.ipa Payload
cd -
# 产物：mobile/shiguang-vps-base.ipa
```

把 `shiguang-vps-base.ipa` 传到 GitHub Release 即可。

### 完整版（含小组件，需付费账号；推荐直接装到自己设备）

完整版的小组件依赖 App Group，**未签名重签对别人不生效**，因此一般是**你自己**用付费账号编译并签名安装：

```bash
cd mobile
# Team ID 在 developer.apple.com → Membership 查；通过环境变量注入，不写进仓库
APPLE_TEAM_ID=<你的10位TeamID> EXPO_WIDGET=1 npx expo prebuild -p ios --clean

# 用 Xcode 打开，给两个 target 选 Team + 确认 App Group 能力，然后 Archive 导出
open ios/VPS.xcworkspace
#   - 选 VPS 与 ExpoWidgetsTarget 两个 target → Signing & Capabilities → Team 选你的付费团队
#   - 确认 App Group group.com.shiguang.vps 已勾选（首次会自动在后台注册）
#   - Product → Archive → Distribute App → (Ad Hoc / Development) → 导出 IPA
```

> 也可用 EAS Build（云端）出包，但仓库未内置 `eas.json`，需自行 `eas build:configure` 并按 `EXPO_WIDGET` 配 build profile 的环境变量。

---

## 四、源码自建（不下 Release，自己从头编到装）

适合想要**完整版小组件**或想改代码的人。

- **基础包到模拟器**（免费、最快验证）：`npx expo run:ios`
- **基础包到真机**（免费 ID 可签 7 天）：`npx expo run:ios --device`
- **完整版（含小组件）到真机**：`APPLE_TEAM_ID=<你的> EXPO_WIDGET=1 npx expo run:ios --device`（**需付费账号**）
- **小组件也可在模拟器测**（免费）：`EXPO_WIDGET=1 npx expo run:ios`（不带 `--device`）

---

## 五、用户自签安装（基础版 IPA）

用 **Sideloadly**（Win/Mac）或 **AltStore**，用自己的 Apple ID 重签 + 安装：

1. iPhone 用数据线连电脑，打开 Sideloadly / AltStore。
2. 选 `shiguang-vps-base.ipa`，填自己的 **Apple ID**（建议用「App 专用密码」）。
3. 点 Start → 工具会用你的「Apple Development（个人团队）」证书重签并安装。
4. iPhone → 设置 → 通用 → **VPN 与设备管理** → 信任你的开发者证书。
5. 打开 App 即可使用。

**有效期**：
- **免费 Apple ID**：签名 **7 天**到期，需重签；且同一免费账号最多 3 个自签 App、设备注册数有上限。
- **付费开发者账号**：签名 **1 年**，100 台设备。

---

## 六、证书类型对照

| 账号类型 | 证书 | 有效期 | App Group / 推送 | 设备数 | 能签哪个包 |
|----------|------|--------|------------------|--------|-----------|
| 免费 Apple ID（个人团队） | Apple Development | 7 天 | ❌ | 受限 | **基础版** |
| 付费 Apple Developer（$99/年） | Apple Development / Distribution | 1 年 | ✅ | 100 | 基础版 + **完整版** |

---

## 七、常见报错

- `does not support the App Groups / Push Notifications capability` / `doesn't support the group.com.shiguang.vps App Group`：你在用**免费账号**编/签**完整版**。改用付费账号，或改编**基础版**（去掉 `EXPO_WIDGET=1` 并 `npx expo prebuild -p ios --clean` 重新生成）。
- `reached the maximum number of registered iPhone devices`：账号注册设备已满。付费账号去 developer.apple.com → Devices 删旧设备；免费账号需换号或等额度释放。
- 去掉 `EXPO_WIDGET=1` 后仍报 App Group 错：旧的 `ios/` 工程还在被复用。必须 **`npx expo prebuild -p ios --clean`** 重新生成。
- SSH 弹「不支持，需要 Dev Build」：SSH 需**真机** + Dev Build；当前底层 SSH 原生库较旧，若仍不可用属已知问题（见仓库 Issue）。

> Android 不受以上限制：直接下 APK 安装即可（见 Android 打包说明）。
