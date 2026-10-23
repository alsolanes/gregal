import java.io.File

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// El backend de Gregal (el mateix servidor Go de l'escriptori) viatja dins
// l'APK com a libgregal.so per ABI: l'app l'executa com a procés fill, com
// fa l'Electron amb gregal.exe. Va a jniLibs perquè Android només deixa
// executar binaris de la carpeta de llibreries natives de l'app.
// -Pgregal.skipBackend salta la compilació (l'app només podrà fer servir
// un servidor remot).
val repoRoot: File = rootDir.parentFile
val backendOut = layout.buildDirectory.dir("gregal-backend/jniLibs")
val backendAbis = linkedMapOf(
    "arm64-v8a" to ("arm64" to "aarch64-linux-android"),
    "x86_64" to ("amd64" to "x86_64-linux-android"),
)

android {
    namespace = "xyz.solanes.gregal"
    compileSdk = 34

    defaultConfig {
        applicationId = if (project.hasProperty("gregal.plotsPreview")) "xyz.solanes.gregal.vents" else "xyz.solanes.gregal"
        manifestPlaceholders["gregalAppLabel"] = if (project.hasProperty("gregal.plotsPreview")) "Gregal Vents" else "Gregal"
        minSdk = 26
        targetSdk = 34
        versionCode = if (project.hasProperty("gregal.plotsPreview")) 98 else 44
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        versionName = if (project.hasProperty("gregal.plotsPreview")) "0.11.0-preview.58" else "0.11.2"
    }

    sourceSets["main"].jniLibs.srcDir(backendOut)
    // Un APK per ABI (app-<abi>-debug.apk) perquè cada mòbil només baixi el
    // seu backend, més l'universal (app-universal-debug.apk) per a qui no
    // sàpiga quin li toca. Tots comparteixen versionCode i signatura: el
    // d'un ABI s'instal·la damunt de l'universal i a l'inrevés.
    splits {
        abi {
            isEnable = true
            reset()
            include(*backendAbis.keys.toTypedArray())
            isUniversalApk = true
        }
    }
    packaging {
        // Extretes al disc (nativeLibraryDir): és l'únic lloc des d'on el
        // sistema deixa fer exec del backend.
        jniLibs.useLegacyPackaging = true
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    testOptions {
        // Robolectric llegeix el manifest i els recursos fusionats: sense
        // això createComposeRule() no troba cap activity llançable.
        unitTests.isIncludeAndroidResources = true
    }
    kotlinOptions {
        jvmTarget = "17"
    }
    buildFeatures {
        compose = true
    }
    composeOptions {
        kotlinCompilerExtensionVersion = "1.5.14"
    }
}

/** L'NDK més nou de l'SDK (per compilar el backend amb cgo: DNS del sistema). */
fun ndkHostBin(): File? {
    val sdk = android.sdkDirectory
    val ndks = File(sdk, "ndk").listFiles()?.filter { it.isDirectory }?.sortedBy { it.name } ?: return null
    val ndk = ndks.lastOrNull() ?: return null
    val os = System.getProperty("os.name").lowercase()
    val host = when {
        os.contains("win") -> "windows-x86_64"
        os.contains("mac") -> "darwin-x86_64"
        else -> "linux-x86_64"
    }
    return File(ndk, "toolchains/llvm/prebuilt/$host/bin").takeIf { it.isDirectory }
}

val buildGregalBackend by tasks.registering {
    description = "Cross-compiles the Gregal Go backend for each Android ABI into jniLibs."
    onlyIf { !project.hasProperty("gregal.skipBackend") }
    inputs.files(fileTree(repoRoot) {
        include("*.go", "go.mod", "go.sum", "internal/**", "harness/**")
        exclude("**/*_test.go", "**/testdata/**")
    })
    outputs.dir(backendOut)
    doLast {
        val win = System.getProperty("os.name").lowercase().contains("win")
        val ndkBin = ndkHostBin()
        if (ndkBin == null) logger.warn("gregal: no Android NDK found, building the backend without cgo (fallback DNS resolver)")
        for ((abi, target) in backendAbis) {
            val (goArch, triple) = target
            val out = backendOut.get().dir(abi).file("libgregal.so").asFile
            out.parentFile.mkdirs()
            val cc = ndkBin?.let { File(it, "${triple}26-clang" + if (win) ".cmd" else "") }
            val pb = ProcessBuilder(
                "go", "build", "-trimpath", "-buildvcs=false", "-ldflags", "-s -w",
                "-o", out.absolutePath, ".",
            ).directory(repoRoot).redirectErrorStream(true)
            val env = pb.environment()
            env["GOOS"] = "android"
            env["GOARCH"] = goArch
            if (cc != null && cc.exists()) {
                env["CGO_ENABLED"] = "1"
                env["CC"] = cc.absolutePath
            } else {
                env["CGO_ENABLED"] = "0"
            }
            val proc = pb.start()
            val log = proc.inputStream.bufferedReader().readText()
            if (proc.waitFor() != 0) throw GradleException("go build for $abi failed:\n$log")
            logger.lifecycle("gregal: backend $abi -> ${out.length() / 1024 / 1024} MB")
        }
    }
}

tasks.matching { it.name.matches(Regex("merge.*JniLibFolders")) }.configureEach {
    dependsOn(buildGregalBackend)
}

dependencies {
    implementation(platform("androidx.compose:compose-bom:2024.05.00"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-extended")
    implementation("androidx.activity:activity-compose:1.9.0")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.0")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.0")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.robolectric:robolectric:4.14.1")
    testImplementation("androidx.compose.ui:ui-test-junit4")

    debugImplementation("androidx.compose.ui:ui-test-manifest")
}
