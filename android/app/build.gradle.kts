import java.io.File
import java.util.concurrent.ConcurrentLinkedQueue

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
    id("org.jetbrains.kotlin.plugin.serialization")
}

// El backend de Gregal (el mateix servidor Go de l'escriptori) viatja dins
// l'APK com a libgregal.so per ABI: l'app l'executa com a procés fill, com
// fa l'Electron amb gregal.exe. Va a jniLibs perquè Android només deixa
// executar binaris de la carpeta de llibreries natives de l'app.
// -Pgregal.skipBackend salta la compilació (l'app només podrà fer servir
// un servidor remot).
val repoRoot: File = rootDir.parentFile
val backendOut = layout.buildDirectory.dir("gregal-backend/jniLibs")
val allBackendAbis = linkedMapOf(
    "arm64-v8a" to ("arm64" to "aarch64-linux-android"),
    "x86_64" to ("amd64" to "x86_64-linux-android"),
)
// -Pgregal.abi=arm64-v8a (o x86_64 per a l'emulador): per provar, compila el
// backend i l'APK només d'aquella arquitectura, sense l'universal. Els
// releases no el passen i surten totes.
val onlyAbi = (findProperty("gregal.abi") as String?)?.takeIf { it.isNotBlank() }
val backendAbis = if (onlyAbi == null) allBackendAbis
    else LinkedHashMap(allBackendAbis.filterKeys { it == onlyAbi }).also {
        require(it.isNotEmpty()) { "gregal.abi ha de ser un de: ${allBackendAbis.keys}" }
    }

// Push FCM (B7), opcional i per a forks: una sola propietat de Gradle,
// gregal.fcm.config = camí al google-services.json del teu projecte de
// Firebase (a ~/.gradle/gradle.properties o -P; mai al repositori). Sense
// ella, l'APK és la de sempre (sense Firebase): avisos per la connexió de
// primer pla i la comprovació periòdica (WindWatch). El servidor necessita,
// a part, GREGAL_FCM_CREDENTIALS. Cap projecte, paquet ni clau nostres aquí.
val fcmConfig: Map<String, String> = run {
    val path = (findProperty("gregal.fcm.config") as String?)?.trim().orEmpty()
    if (path.isEmpty()) return@run emptyMap()
    val file = File(path)
    require(file.isFile) { "gregal.fcm.config: no existeix $path" }
    @Suppress("UNCHECKED_CAST")
    val json = groovy.json.JsonSlurper().parse(file) as Map<String, Any?>
    val info = json["project_info"] as? Map<String, Any?> ?: emptyMap()
    @Suppress("UNCHECKED_CAST")
    val clients = (json["client"] as? List<Map<String, Any?>>).orEmpty()
    // El client de l'ID de paquet d'aquesta build; si no n'hi ha, el primer.
    fun pkg(c: Map<String, Any?>) = ((c["client_info"] as? Map<*, *>)?.get("android_client_info") as? Map<*, *>)?.get("package_name")
    val wanted = if (project.hasProperty("gregal.plotsPreview")) "xyz.solanes.gregal.vents" else "xyz.solanes.gregal"
    val client = clients.firstOrNull { pkg(it) == wanted } ?: clients.firstOrNull() ?: emptyMap()
    val appId = (client["client_info"] as? Map<*, *>)?.get("mobilesdk_app_id")?.toString().orEmpty()
    val apiKey = ((client["api_key"] as? List<*>)?.firstOrNull() as? Map<*, *>)?.get("current_key")?.toString().orEmpty()
    mapOf(
        "projectId" to info["project_id"]?.toString().orEmpty(),
        "appId" to appId,
        "apiKey" to apiKey,
        "senderId" to info["project_number"]?.toString().orEmpty(),
    ).also { m -> require(m.values.all { it.isNotEmpty() }) { "gregal.fcm.config: al fitxer hi falten camps de Firebase" } }
}
val fcmOn = fcmConfig.isNotEmpty()

android {
    namespace = "xyz.solanes.gregal"
    compileSdk = 36

    defaultConfig {
        applicationId = if (project.hasProperty("gregal.plotsPreview")) "xyz.solanes.gregal.vents" else "xyz.solanes.gregal"
        manifestPlaceholders["gregalAppLabel"] = if (project.hasProperty("gregal.plotsPreview")) "Gregal Vents" else "Gregal"
        minSdk = 26
        targetSdk = 34
        versionCode = if (project.hasProperty("gregal.plotsPreview")) 198 else 49
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        versionName = if (project.hasProperty("gregal.plotsPreview")) "1.75.0-vents" else "1.62.0"
        manifestPlaceholders["gregalFcm"] = fcmOn.toString()
        if (fcmOn) fcmConfig.forEach { (k, v) -> buildConfigField("String", "FCM_" + k.uppercase(), "\"" + v.replace("\"", "") + "\"") }
    }

    // L'APK surt amb el nom de l'app i la versió: «Gregal-Vents-<versió>-arm64-v8a.apk» no
    // deia ni quina app era ni quina versió, i al mòbil totes es deien igual.
    applicationVariants.all {
        val etiqueta = if (project.hasProperty("gregal.plotsPreview")) "Gregal-Vents-" else "Gregal-Android-"
        val base = etiqueta + versionName.orEmpty().removeSuffix("-vents")
        outputs.all {
            val abi = filters.joinToString("-") { it.identifier }
            (this as com.android.build.gradle.internal.api.BaseVariantOutputImpl).outputFileName =
                if (abi.isBlank()) "$base.apk" else "$base-$abi.apk"
        }
    }

    sourceSets["main"].jniLibs.srcDir(backendOut)
    if (fcmOn) sourceSets["main"].java.srcDir("src/fcm/java")
    // Un APK per ABI (app-<abi>-debug.apk) perquè cada mòbil només baixi el
    // seu backend, més l'universal (Gregal-Vents-<versió>.apk) per a qui no
    // sàpiga quin li toca. Tots comparteixen versionCode i signatura: el
    // d'un ABI s'instal·la damunt de l'universal i a l'inrevés.
    splits {
        abi {
            isEnable = true
            reset()
            include(*backendAbis.keys.toTypedArray())
            isUniversalApk = onlyAbi == null
        }
    }
    packaging {
        // Extretes al disc (nativeLibraryDir): és l'únic lloc des d'on el
        // sistema deixa fer exec del backend.
        jniLibs.useLegacyPackaging = true
    }

    buildTypes {
        debug {
            if (project.hasProperty("gregal.scrollTest")) applicationIdSuffix = ".scrolltest"
        }
        // La build que es publica. Abans sortia l'APK de debug: una app
        // «debuggable» corre Compose sense R8 ni els perfils de referència de
        // les llibreries (ART no la precompila), i al mòbil es notava lenta.
        // Signa amb la mateixa clau que les de debug perquè l'OTA s'instal·li
        // damunt de les versions que ja hi ha als mòbils.
        release {
            signingConfig = signingConfigs.getByName("debug")
            isMinifyEnabled = true
            isShrinkResources = true
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
    buildFeatures {
        compose = true
        buildConfig = fcmOn
    }
}

// Strong skipping ja ve encès per defecte amb el compilador de Compose de
// Kotlin 2: els composables amb paràmetres inestables (llistes, lambdes que
// capturen el vm) se salten si no han canviat.
kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
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
    // Si canvia la llista d'arquitectures (-Pgregal.abi), cal tornar-la a fer.
    inputs.property("abis", backendAbis.keys.toList())
    outputs.dir(backendOut)
    doLast {
        // Fora les arquitectures que aquesta build no porta (d'una build anterior).
        backendOut.get().asFile.listFiles()?.filter { it.name !in backendAbis.keys }?.forEach { it.deleteRecursively() }
        val win = System.getProperty("os.name").lowercase().contains("win")
        val ndkBin = ndkHostBin()
        if (ndkBin == null) logger.warn("gregal: no Android NDK found, building the backend without cgo (fallback DNS resolver)")
        // Les arquitectures es compilen alhora (abans, una darrere l'altra).
        val failures = ConcurrentLinkedQueue<String>()
        val threads = backendAbis.map { (abi, target) -> Thread {
          try {
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
            if (proc.waitFor() != 0) failures.add("go build for $abi failed:\n$log")
            else logger.lifecycle("gregal: backend $abi -> ${out.length() / 1024 / 1024} MB")
          } catch (e: Exception) { failures.add("go build for $abi failed: ${e.message}") }
        } }
        threads.forEach { it.start() }
        threads.forEach { it.join() }
        if (failures.isNotEmpty()) throw GradleException(failures.joinToString("\n"))
    }
}

tasks.matching { it.name.matches(Regex("merge.*JniLibFolders")) }.configureEach {
    dependsOn(buildGregalBackend)
}

dependencies {
    implementation(platform("androidx.compose:compose-bom:2026.06.01"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-extended")
    implementation("androidx.activity:activity-compose:1.9.0")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.0")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.0")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.12.0")
    implementation("com.squareup.okhttp3:okhttp:5.4.0")
    implementation("com.squareup.okhttp3:okhttp-sse:5.4.0")
    if (fcmOn) implementation("com.google.firebase:firebase-messaging:24.1.2")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.robolectric:robolectric:4.14.1")
    testImplementation("androidx.compose.ui:ui-test-junit4")

    androidTestImplementation(platform("androidx.compose:compose-bom:2026.06.01"))
    androidTestImplementation("androidx.compose.ui:ui-test-junit4")
    androidTestImplementation("androidx.test.ext:junit:1.2.1")
    debugImplementation("androidx.compose.ui:ui-test-manifest")
}
