// React Native autolinking overrides.
//
// react-native-ssh-sftp@1.0.3 is abandoned (peer react-native ^0.54.0). Its
// android/build.gradle still uses AGP 1.3.1 / jcenter() / `compile` / SDK 23,
// which fails to compile under this project's RN 0.85 + AGP 8 toolchain and
// breaks the whole Android `gradlew` build. On iOS it ships no podspec, so it
// is never autolinked there either.
//
// SSH native integration is therefore non-functional on both platforms today;
// the JS layer lazy-`require`s the module inside try/catch and no-ops when the
// native side is absent. So we exclude it from autolinking entirely — this
// unblocks the Android build with zero behavioral loss. When we migrate to a
// maintained fork (e.g. @dylankenneally/react-native-ssh-sftp), drop this entry.
module.exports = {
  dependencies: {
    "react-native-ssh-sftp": {
      platforms: {
        android: null,
        ios: null,
      },
    },
  },
};
