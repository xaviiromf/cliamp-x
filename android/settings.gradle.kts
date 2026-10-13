pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
        // The Termux terminal libraries are published only on JitPack.
        maven("https://jitpack.io") {
            content { includeGroup("com.github.termux.termux-app") }
        }
    }
}

rootProject.name = "cliamp-x"
include(":app")
