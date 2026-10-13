import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// The one ABI this APK carries: arm64-v8a for phones, x86_64 for the emulator
// (-PcliampAbi=x86_64). make android-core builds the Go core for it.
val cliampAbi = providers.gradleProperty("cliampAbi").getOrElse("arm64-v8a")

fun git(vararg args: String): String =
    providers.exec { commandLine("git", *args) }.standardOutput.asText.get().trim()

android {
    namespace = "io.github.xaviiromf.cliampx"
    compileSdk = 36

    defaultConfig {
        applicationId = "io.github.xaviiromf.cliampx"
        minSdk = 29
        targetSdk = 35
        // Commits on the branch only grow, so the count orders every build.
        versionCode = git("rev-list", "--count", "HEAD").toInt()
        versionName = providers.gradleProperty("cliampVersion")
            .getOrElse(git("describe", "--tags", "--always")).removePrefix("v")
        ndk { abiFilters += cliampAbi }
        // The updater fetches cliamp-x-<version>-<ABI>.apk.
        buildConfigField("String", "ABI", "\"$cliampAbi\"")
    }

    // Release signing comes from the environment, as nightly.yml sets it. Without
    // ANDROID_KEYSTORE_FILE, assembleRelease builds app-release-unsigned.apk.
    // Only APKs signed with the same key update each other.
    val keystore = providers.environmentVariable("ANDROID_KEYSTORE_FILE").orNull
    signingConfigs {
        if (keystore != null) create("release") {
            storeFile = file(keystore)
            storePassword = providers.environmentVariable("ANDROID_KEYSTORE_PASSWORD").get()
            keyAlias = providers.environmentVariable("ANDROID_KEY_ALIAS").get()
            keyPassword = providers.environmentVariable("ANDROID_KEY_PASSWORD").get()
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.findByName("release")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        buildConfig = true
    }

    // The Go core, Python and ffmpeg run as programs from nativeLibraryDir,
    // so the .so files must be extracted at install (extractNativeLibs).
    packaging {
        jniLibs {
            useLegacyPackaging = true
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

val checkCore by tasks.registering {
    val core = file("src/main/jniLibs/$cliampAbi/libcliamp.so")
    val hint = if (cliampAbi == "arm64-v8a") "make android-core" else "make android-core ANDROID_ABI=$cliampAbi"
    doLast {
        if (!core.isFile) throw GradleException("$core is missing: run $hint from the repository root")
    }
}

tasks.named("preBuild") { dependsOn(checkCore) }

dependencies {
    implementation("androidx.core:core-ktx:1.18.0")
    implementation("androidx.appcompat:appcompat:1.8.0")
    implementation("androidx.media:media:1.8.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.11.0")
    implementation("com.github.termux.termux-app:terminal-view:v0.118.3")
    implementation("com.github.termux.termux-app:terminal-emulator:v0.118.3")
    implementation("io.github.junkfood02.youtubedl-android:library:0.18.1")
    implementation("io.github.junkfood02.youtubedl-android:ffmpeg:0.18.1")
    testImplementation("junit:junit:4.13.2")
    // The real org.json: android.jar has only stubs on the JVM.
    testImplementation("org.json:json:20260814")
}
