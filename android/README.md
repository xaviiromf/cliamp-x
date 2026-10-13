# cliamp-x for Android

The Android shell of cliamp-x: a Kotlin app that runs the Go core in a
terminal view. The design is in `PLAN.md`.

## Toolchain

| Tool | Version | Where this machine has it |
|------|---------|---------------------------|
| JDK | 17 | `mise install java@temurin-17`, then `export JAVA_HOME=$(mise where java@temurin-17)` |
| Android SDK | platform 36, build-tools 35.0.0, platform-tools | `~/Android/Sdk` |
| Android NDK | r28c (`28.2.13676358`) | `~/Android/Sdk/ndk/28.2.13676358` |
| Emulator | API 36 `google_apis` x86_64 image | `~/Android/Sdk/emulator` |

Install the SDK pieces with the command-line tools:

```
sdkmanager "platform-tools" "platforms;android-36" "build-tools;35.0.0" "ndk;28.2.13676358"
```

`android/local.properties` holds `sdk.dir` and stays out of git. Set
`ANDROID_HOME` and `ANDROID_NDK_HOME` in the shell for `make android-core`.
