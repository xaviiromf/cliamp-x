# cliamp-x para Android: plan de implementación

Plan para un agente (Opus 5.5, xhigh, ultracode). Dispositivo objetivo: Samsung
Galaxy A55 5G (SM-A556B, Exynos 1480, arm64-v8a, 8 GB, pantalla 1080x2340 a
~2.75x, unos 390x845 dp) con Android 16 / One UI 8. Las decisiones de la
sección 1 ya están tomadas: no las vuelvas a evaluar, ejecútalas. Si algo de la
sección 1 resulta imposible, para y explícalo antes de buscar otro camino.

Lee antes de empezar: `CLAUDE.md`, `docs/remote-control.md`, `docs/headless.md`,
`docs/youtube-music.md`, `main.go` (`run`, `programOptions`, `quitOnSignals`,
`v2Operations`), `ui/model/layout.go`, `ui/model/mouse.go`, `ui/model/init.go`
(`SetHeadless`), `player/ytdl.go`, `player/ffmpeg.go`, `upgrade/upgrade.go`.

---

## 1. Decisiones cerradas

| Tema | Decisión | Motivo |
|------|----------|--------|
| UI | La misma TUI Bubbletea, dibujada por la vista de terminal de Termux (`terminal-emulator` + `terminal-view`, Apache 2) dentro de una Activity Kotlin | Mantiene estética, temas, visualizadores y plugins al 100%. Nada de Compose, Flutter ni reescritura |
| UI (reemplaza la fila anterior) | Interfaz nativa táctil tipo Spotify con la estética de cliamp-x, en Jetpack Compose, según `android/UI-SPEC.md` (hecha: hitos M1 a M6 de su §10). Solo en vertical. La TUI queda como "Modo terminal", que sí gira | Lo pidió el usuario tras probar la TUI en el teléfono: poco intuitiva y sin teclado no se usa cómodo |
| Núcleo | El mismo módulo Go, compilado con el NDK como ejecutable PIE llamado `libcliamp.so`, empaquetado en `jniLibs/arm64-v8a/` y ejecutado desde `nativeLibraryDir` en un pty | Android 10+ solo permite `exec` desde `nativeLibraryDir`. Es lo que hace Termux. Sin gomobile |
| yt-dlp y ffmpeg | Librería `youtubedl-android` (`io.github.junkfood02.youtubedl-android:library` y `:ffmpeg`), que empaqueta Python, yt-dlp y ffmpeg | Única forma probada de tener yt-dlp en un APK. Además permite actualizar yt-dlp sin reinstalar la app |
| Audio | `beep` sobre `oto` v3, cuyo `driver_android.go` usa Oboe | Ya es el backend actual. Un proceso hijo del mismo UID puede abrir audio |
| Controles del sistema | Foreground service con `MediaSessionCompat` (androidx.media) + notificación `MediaStyle`, hablando con el núcleo por el socket IPC V2 ya existente | Sustituye MPRIS y `desktopnotify` sin tocar el Model. Menos código que Media3 |
| Login YouTube Music | WebView en Kotlin que exporta las cookies a `ytmusic_cookies.txt` (formato Netscape) en el directorio de config. Plan B obligatorio: importar un `cookies.txt` con el selector de archivos del sistema | El Go ya consume ese archivo (`internal/ytdlcookies`, `credstore`) |
| Actualizaciones | Menú "Buscar actualizaciones" que consulta GitHub Releases, descarga el APK, verifica SHA-256 y abre el instalador del sistema. Canal estable (tags `v*`) y canal nightly (prerelease `nightly` desde `main`). Nunca sustituir el binario Go en caliente | Android bloquea ejecutar binarios descargados. Play Store queda fuera por política y por yt-dlp |
| Distribución | GitHub Releases, compatible con Obtainium. ABI solo `arm64-v8a` | Es el A55. Añadir otras ABIs es trivial después |
| Organización | Rama `android` en este repo, directorio `android/` para el shell Kotlin, build tags `android` en Go. No es un fork | Cada cambio de `main` llega al APK con el siguiente tag |
| SDK | `minSdk 29`, `targetSdk 35`, `compileSdk 36`, JDK 17, AGP 8.x, Kotlin 2.x, NDK r27 o superior | API 29 es donde empieza W^X, no tiene sentido ir más abajo. targetSdk 35 evita las novedades de 36 en la primera versión |

---

## 1b. Rama de trabajo y cómo le llegan los cambios de `main`

- Todo el trabajo va en la rama `android`, creada desde `main`. No se toca `main` directamente. Crear la rama al empezar la fase 0: `git switch -c android main`.
- `main` sigue siendo la fuente de verdad del player. Cada vez que `main` avance, actualizar la rama con `git merge main` (nunca rebase: la rama es pública y larga). Resolver conflictos a favor de `main` en todo lo que no sea código Android.
- Para que ese merge sea casi siempre limpio, el código Go nuevo de este plan vive en archivos nuevos (`internal/extbin/`, `*_android.go`, la operación `ui.render`) o en cambios de una línea (build tags, `runtime.GOOS == "android"`). Nada de reorganizar archivos existentes.
- La app Android se actualiza así: merge de `main` en `android`, tag `v*` en `android`, y `release.yml` publica el APK. Un tag en `main` publica solo los paquetes Linux hasta que la rama se integre.
- Objetivo final: cuando las fases 1 a 5 estén verdes en el A55 y `make ci` pase, abrir un PR de `android` a `main` y fusionar. A partir de ahí no hay rama paralela: cada tag de `main` produce Linux y APK. El agente propone ese PR al terminar; la decisión de fusionar es del usuario.
- Cosas de `main` que afectan al APK sin tocar `android/`: cambios en `docs/remote-control.md` (el cliente IPC Kotlin los sigue), en claves de `config.toml.example` (la config inicial que escribe el shell) y en `command_registry.go` (la fila de teclas extra). Revisarlos en cada merge.

---

## 2. Hechos del código que condicionan el trabajo

- `GOOS=android` activa también el tag `linux`. Todo `//go:build linux` compila en Android. Hay que excluirlo a mano: `mediactl/service_linux.go`, `player/audio_device_linux.go`, `player/audio_hint_linux.go`, `cmd/protocol_linux.go`. Sus stubs (`service_stub.go`, `audio_device_stub.go`, `audio_hint_stub.go`, `protocol_other.go`) deben incluir `android`.
- `yt-dlp` y `ffmpeg` se invocan por nombre con `exec.Command` en: `player/ytdl.go` (líneas ~38, ~57, ~306), `player/ytdl_info.go`, `player/ffmpeg.go` (~242, ~454, ~544, ~659), `internal/offline/offline.go` (~184, ~206, ~296), `resolve/ytdl_playlists.go`, `luaplugin/api_exec.go` (`defaultAllowedBinaries`). En Android yt-dlp no es un ejecutable, es `libpython.so` más argumentos y variables de entorno. Hace falta un único punto de resolución.
- `ui/model/layout.go`: `width < 40 || height < 10` es "demasiado pequeño"; `>= 56x16` es `layoutCompact`; `>= 80x24` es `layoutFull`; el resto es `layoutMinimal`. El A55 en vertical con fuente de 10 sp da unos 60-65 columnas: tier compacto. En horizontal, tier completo.
- `ui/model/mouse.go` ya trata clics, rueda y clic derecho (menú de fila). La vista de Termux manda ratón SGR cuando el programa lo activa.
- `SetHeadless` en `ui/model/init.go` hace que `View` devuelva vacío y baja el tick, pero no es reversible. Hace falta una variante reversible por IPC para cuando la app pasa a segundo plano.
- `quitOnSignals` en `main.go` convierte SIGHUP en salida con guardado. El pty no debe mandar HUP al cerrar la Activity, solo al parar el service.
- `ui/artwork/kitty.go` detecta kitty por `TERM`. La vista de Termux no soporta gráficos kitty; con `TERM=xterm-256color` cae al medio bloque, que es lo correcto.
- `upgrade/upgrade.go` sustituye el ejecutable (`replaceExecutable`). En Android eso no puede funcionar. La lógica de versión (`version.go`), `latestRelease` y `releaseChecksum` sí sirven, pero son privadas.
- `providers.go` ofrece instalar yt-dlp y abre el navegador para el login. Ambas cosas deben desactivarse en Android (igual que en headless).
- `internal/desktopnotify` y `mediactl/metadata.go` usan godbus. godbus es Go puro: compila, pero debe no intentarse en Android.
- `.github/workflows/release.yml` compila con una matriz Linux y nfpm en tags `v*`. Los runners `ubuntu-latest` traen SDK y NDK de Android en `$ANDROID_HOME` y `$ANDROID_NDK_LATEST_HOME`.
- Directorios: `internal/appdir` lee `CLIAMP_CONFIG_DIR`, `XDG_CONFIG_HOME` y `HOME`. `os.TempDir` en Android usa `TMPDIR` y por defecto apunta a `/data/local/tmp`, que no es escribible. El shell debe fijar `TMPDIR`.

---

## 3. Fases

Las fases 1, 2 y 3 son independientes entre sí y pueden ir en paralelo con
agentes distintos. La fase 4 depende de 1 y 2. La fase 5 cierra. Reparto de
archivos para que los agentes no choquen:

| Agente | Posee |
|--------|-------|
| A (Go núcleo) | build tags, `internal/extbin/`, `player/`, `internal/offline/`, `resolve/`, `luaplugin/api_exec.go`, `providers.go`, `main.go`, `ipc/`, `ui/model/ipc_runtime.go`, `upgrade/`, `commands.go` |
| B (shell Android) | `android/` completo |
| C (pantalla pequeña) | `ui/model/layout.go`, `view*.go`, `playlist_rows.go`, `view_bench_test.go` y tests nuevos de layout |
| D (updater, tras A y B) | `android/.../update/`, el comando `upgrade` en `commands.go`, `.github/workflows/` |

### Fase 0. Entorno (bloquea el resto; pide al usuario lo marcado con ⚠)

1. Instalar en la máquina de desarrollo: JDK 17, Android cmdline-tools, `platforms;android-36`, `build-tools;35.0.0`, `ndk;27.2.12479018` o posterior, `platform-tools`. Usar `sdkmanager --licenses`. Dejar `ANDROID_HOME` y `ANDROID_NDK_HOME` en `android/local.properties` (ignorado por git) y documentarlo en `android/README.md`.
2. ⚠ Pedir al usuario que active en el A55 Opciones de desarrollador (7 toques en Número de compilación) y Depuración USB, y que conecte el cable o active Depuración inalámbrica. Comprobar con `adb devices`.
3. Verificar que `mise exec -- go version` da 1.26.9 y que compila el núcleo para Android (comando de la fase 1, paso 6) antes de tocar Kotlin.
4. ⚠ Keystore: generar `android/keystore/release.jks` con `keytool`, fuera de git. Explicar al usuario que si pierde esa clave ningún APK futuro podrá actualizar a los ya instalados. Guardar base64 y contraseñas como secretos de GitHub `ANDROID_KEYSTORE_B64`, `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_ALIAS`, `ANDROID_KEY_PASSWORD`.

### Fase 1. El núcleo Go compila y arranca en Android (agente A)

1. **Build tags.** Cambiar `//go:build linux` a `//go:build linux && !android` en los cuatro archivos listados en la sección 2. Añadir `|| android` donde haga falta a los stubs para que cada símbolo exista exactamente una vez en `GOOS=android`. Confirmar con `GOOS=android GOARCH=arm64 go vet ./...` (sin cgo: `CGO_ENABLED=0` para vet es suficiente para los tags; el build real es con cgo).
2. **`internal/extbin`** (paquete nuevo, pequeño): `YTDLP(ctx, args...) *exec.Cmd`, `FFmpeg(args...) *exec.Cmd`, `HaveYTDLP() bool`, `HaveFFmpeg() bool`. Resolución: si `CLIAMP_YTDLP` o `CLIAMP_FFMPEG` está definida, es un argv separado por espacios (los directorios de una app Android no llevan espacios; `shortcut:` sin comillas, actualizar si algún día hace falta); si no, `exec.LookPath` del nombre como hoy. Las variables `CLIAMP_YTDLP_ENV` y `CLIAMP_FFMPEG_ENV` opcionales con pares `K=V` separados por `\x1f` se añaden a `cmd.Env` (Python necesita `PYTHONHOME`, `LD_LIBRARY_PATH`, `SSL_CERT_FILE`). Sustituir todas las llamadas directas listadas en la sección 2, incluida la lista blanca de `luaplugin/api_exec.go` (debe aceptar el ejecutable resuelto). Tests table-driven de la resolución. Mover `InstallHint` de `internal/ytdlp` o dejarlo: lo que dé el diff más corto.
3. **Android en `providers.go` y `main.go`.** Donde hoy se consulta `headless` para no abrir el navegador ni ofrecer instalar yt-dlp, añadir `runtime.GOOS == "android"`. `desktopnotify.New` y `mediactl.New` no se llaman en Android (el stub de `mediactl` ya devuelve nil; `desktopnotify` necesita el guard). `internal/browser.Open` en Android: escribir la URL en el log y en la barra de estado, nada más.
4. **Operación IPC `ui.render`** con parámetro `on` (bool). Registrarla en `ipc/operations.go`, manejarla en `ui/model/ipc_runtime.go`: `on=false` pone `m.renderPaused = true` (`View` devuelve `""`, `lowPower = true`); `on=true` lo revierte, restaura `lowPower` al valor de config, llama `recomputeLayout` y fuerza un repintado. Quitarla en headless dentro de `v2Operations`. Tests en `ui/model/` e `ipc/`. Documentar en `docs/remote-control.md`.
5. **`upgrade` en Android.** `upgrade/replace_android.go` con `//go:build android` donde `Run` devuelve un error claro ("update from the app menu"). Mantener el stub actual para el resto. Esto deja listo el paso de la fase 4 que exporta la consulta de versión.
6. **Comando de compilación** en el `Makefile`, target `android-core`:
   ```
   GOOS=android GOARCH=arm64 CGO_ENABLED=1 \
   CC=$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android29-clang \
   CXX=$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android29-clang++ \
   go build -trimpath -buildmode=pie \
     -ldflags "-s -w $(VERSION_LDFLAGS) -extldflags '-Wl,-z,max-page-size=16384'" \
     -o android/app/src/main/jniLibs/arm64-v8a/libcliamp.so .
   ```
   `max-page-size=16384` por los kernels de 16 KB que Android 15+ permite. Verificar alineación con `$ANDROID_HOME/ndk/<ver>/.../check_elf_alignment.sh` o `readelf -lW`. oto compila C++ (Oboe): `CXX` es obligatorio.
7. **Prueba en el dispositivo sin APK** (vale para no esperar a la fase 2): `adb push libcliamp.so /data/local/tmp/` y ejecutarlo con `adb shell` dentro de `/data/local/tmp` con `HOME`, `CLIAMP_CONFIG_DIR`, `TMPDIR` apuntando a un subdirectorio de `/data/local/tmp`. Como shell (uid 2000) sí puede ejecutar ahí. Objetivo: `libcliamp.so --version`, luego `libcliamp.so --daemon` con un archivo local de prueba (`adb push` de un mp3) y comprobar que suena. Este paso valida oto/Oboe y modernc sqlite en arm64 antes de escribir Kotlin. Si Oboe falla desde `adb shell` por permisos de audio del uid shell, no concluir nada: repetir dentro del APK en la fase 2.
8. **CI.** Job `android-core` en `ci.yml` que ejecuta `make android-core` con `ANDROID_NDK_HOME=$ANDROID_NDK_LATEST_HOME`. Impide que un futuro `//go:build linux` rompa Android.

Criterio de salida: `make ci` verde, `make android-core` produce el binario, el paso 7 reproduce audio o queda documentado por qué no se pudo probar fuera del APK.

### Fase 2. Shell Android (agente B)

Estructura: `android/settings.gradle.kts`, `android/app/`, módulo único,
paquete `io.github.xaviiromf.cliampx` (o el que diga el usuario). Gradle con
`useLegacyPackaging = true` (equivale a `extractNativeLibs=true`): sin eso los
`.so` no se extraen y no se pueden ejecutar.

Dependencias: `com.github.termux.termux-app:terminal-view` y
`:terminal-emulator` (JitPack; comprobar la última versión publicada y su
licencia Apache 2), `io.github.junkfood02.youtubedl-android:library` y
`:ffmpeg` (última estable), `androidx.media:media`, `androidx.core`,
`androidx.appcompat`, `kotlinx-coroutines-android`. Nada más. HTTP con
`HttpURLConnection`. JSON con `org.json`.

1. **Primer hito: pty + vista.** `MainActivity` con `TerminalView` que lanza `/system/bin/sh` en un `TerminalSession`. Resuelve teclado, insets edge-to-edge (targetSdk 35 lo impone: `WindowCompat.setDecorFitsSystemWindows(false)` y aplicar insets de barra de estado, navegación e IME a la vista), fuente y tamaño. Hasta que esto no funcione en el A55, no seguir.
2. **`CliampService`**, foreground service `android:foregroundServiceType="mediaPlayback"`, permisos `FOREGROUND_SERVICE`, `FOREGROUND_SERVICE_MEDIA_PLAYBACK`, `POST_NOTIFICATIONS` (runtime), `INTERNET`, `WAKE_LOCK`. Es el dueño del `TerminalSession`: la Activity se enlaza (`bindService`) y adjunta la vista, como Termux. Al destruir la Activity no se envía HUP. El service muere solo con la acción "Salir" de la notificación o cuando el proceso Go termina.
3. **Lanzamiento del núcleo.** Comando: `<nativeLibraryDir>/libcliamp.so`. Entorno:
   - `HOME=<filesDir>/home`, `CLIAMP_CONFIG_DIR=<filesDir>/config`, `XDG_DATA_HOME=<filesDir>/data`, `XDG_CACHE_HOME=<cacheDir>`, `TMPDIR=<cacheDir>/tmp` (crear antes).
   - `TERM=xterm-256color`, `COLORTERM=truecolor`, `LANG=es_ES.UTF-8` (o la del sistema), `LINES`/`COLUMNS` los pone el pty.
   - `PATH=<nativeLibraryDir>`.
   - `CLIAMP_FFMPEG=<nativeLibraryDir>/libffmpeg.so` y `CLIAMP_FFMPEG_ENV` con `LD_LIBRARY_PATH` si la librería lo necesita.
   - `CLIAMP_YTDLP` y `CLIAMP_YTDLP_ENV`: copiar exactamente el comando y las variables que `YoutubeDL.java` de youtubedl-android usa internamente (ruta de `libpython.so`, `PYTHONHOME`, `PYTHONPATH`, `LD_LIBRARY_PATH`, `SSL_CERT_FILE`, el zip de yt-dlp). Antes del primer arranque llamar a `YoutubeDL.getInstance().init(context)` para que extraiga Python. Leer el código fuente de la librería, no adivinar.
   - Primer arranque: escribir un `config.toml` mínimo si no existe (`mouse = true` y `music_dir`; sin `low_power`, porque `ui.render off` ya cubre el segundo plano y el visualizador debe verse a su ritmo normal; el núcleo lee `ytmusic_cookies.txt` del directorio de config sin clave propia).
4. **Cliente IPC en Kotlin.** `LocalSocket` en namespace `FILESYSTEM` contra `<config>/cliamp-x.sock`, una línea JSON por petición con `"version": 2`, según `docs/remote-control.md`. Operaciones necesarias: `state.get`, `subscribe`, `playback.*`, `volume`, `seek`, `ui.render`. Reconectar con backoff mientras el proceso arranca.
5. **MediaSessionCompat + notificación `MediaStyle`.** Estado desde `subscribe`. Portada: descargar la URL del `state.get` a bitmap con caché pequeña. Botones: anterior, pausa, siguiente, salir. Audio focus con `AudioManager`: pérdida → pausa; transitoria → pausa y reanuda; `ACTION_AUDIO_BECOMING_NOISY` → pausa. Botones de auriculares y Bluetooth llegan por la sesión. Teclas de volumen del teléfono controlan el volumen del sistema (no el del player).
6. **Segundo plano.** `onStop` de la Activity → `ui.render off`. `onStart` → `ui.render on` y `TerminalView.onScreenUpdated`. Así el proceso no pinta mientras la pantalla está apagada.
7. **Fila de teclas extra** propia (la de Termux no está en la librería): `Esc`, `Tab`, `↑ ↓ ← →`, `Enter`, `Espacio`, `/`, `F1`, `Ctrl` (modo pegajoso), `F5`. Botón atrás físico/gesto → `Esc`; si la TUI está en la cola y no hay overlay, el segundo atrás manda la Activity al fondo (nunca mata el service). Pellizco para cambiar el tamaño de fuente, persistido en `SharedPreferences`. Tamaño por defecto el que dé ≥ 56 columnas en vertical en el A55 (medir, no calcular).
8. **Login de YouTube Music.** `LoginActivity` con WebView, JS y DOM storage activados, user agent de Chrome Android reciente. URL: `https://accounts.google.com/ServiceLogin?continue=https://music.youtube.com/`. Al detectar navegación a `music.youtube.com` con cookie `SAPISID` presente: `CookieManager.getInstance().getCookie(url)` para `https://music.youtube.com`, `https://www.youtube.com`, `https://accounts.google.com`, `https://.google.com`; escribir `ytmusic_cookies.txt` en formato Netscape (`# Netscape HTTP Cookie File`, campos TAB: domain, includeSubdomains, path, secure, expiry, name, value; expiry 0 si no se conoce). Reiniciar el proceso Go desde el service. Si Google rechaza el WebView ("este navegador puede no ser seguro"), el usuario usa el plan B: botón "Importar cookies.txt" con `ACTION_OPEN_DOCUMENT` que copia el archivo al mismo destino.
9. **Permisos de música local.** `READ_MEDIA_AUDIO` runtime. Con él, `/storage/emulated/0/Music` se lee por ruta directa, que es lo que usan `locallib` y `external/local`. Pantalla de ajustes con un botón para concederlo y escribir `[local] music_dir` en la config (clave exacta según `config.toml.example`).
10. **Batería en One UI.** Pantalla de ajustes con botón `ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS` y texto que explique añadir cliamp-x a Batería → Límites de uso en segundo plano → Apps que nunca se suspenden. Samsung mata foreground services que considera inactivos si no está ahí.
11. **Deep links.** Intent filter para `cliamp-x://` y para compartir URLs de `youtube.com` / `music.youtube.com`: el service ejecuta `<nativeLibraryDir>/libcliamp.so open <url>` como proceso hijo con el mismo entorno. Reutiliza `open.go` sin escribir nada nuevo. Comprobar en `open.go` qué URLs acepta.

Criterio de salida: APK debug instalado en el A55 con `adb install`, reproduce una canción de YouTube Music con sesión iniciada, sigue sonando con la pantalla apagada 30 minutos, controles en pantalla de bloqueo y auriculares Bluetooth funcionan, y la app vuelve a la TUI al abrirla.

### Fase 3. UX en pantalla pequeña (agente C, en escritorio)

1. Lanzar la TUI real en un pty con HOME aislado (ver CLAUDE.md) a 60x32 (vertical) y 110x30 (horizontal) con `tmux` o `script`, capturar la pantalla y revisar cada pantalla del `command_registry`: cola, explorador, ajustes, EQ (`E`), estadísticas (`S`), descargas (`Ctrl+O`), letras, menú de acciones (`F1`) y menú de fila (`O`). Anotar todo recorte, solapamiento o texto cortado en el tier compacto.
2. Arreglar en `layout.go` y `view*.go` lo encontrado. Prioridad: que el menú de acciones y el menú de fila quepan y se puedan tocar, que la barra de ayuda muestre pocas pistas (`maxHelpHints`) y que la lista tenga filas de una línea con objetivo táctil claro.
3. Añadir un test que renderice `View` a 60x32 y 110x30 con datos de ejemplo y compruebe que ninguna línea supera el ancho ni hay más líneas que el alto. Hoy `view_bench_test.go` solo compara el frame completo.
4. Comprobar que cada acción de una tecla está en `actionGroups` (`keymap.go`); la que no esté cae en "Navigation", así que listarlas y agruparlas bien. En el móvil el menú `F1` es el teclado.
5. No tocar la lógica de ratón salvo que un objetivo táctil falle; si falla, arreglarlo en `mouse.go` para todas las plataformas.

Criterio de salida: `make check` verde, capturas antes y después en el PR.

### Fase 4. Actualizaciones (agente D)

1. **Go:** flag `--check --json` en el comando `upgrade` (`commands.go`, `upgrade/`) que imprime `{"current","latest","prerelease","assets":[{"name","url","sha256"}]}` sin descargar nada, en todas las plataformas. Exportar lo mínimo de `upgrade` para ello (`Check(current string, prerelease bool) (Report, error)`). El SHA-256 sale del archivo de checksums que la release ya publica (`releaseChecksum`). Tests table-driven con un servidor HTTP falso, como los existentes.
2. **Kotlin:** `UpdateChecker` que ejecuta `libcliamp.so upgrade --check --json [--prerelease]` como proceso hijo, parsea, compara con `BuildConfig.VERSION_NAME`, descarga el asset `cliamp-x-<ver>-arm64-v8a.apk` a `cacheDir/updates/`, verifica SHA-256 y lanza la instalación con `ACTION_VIEW` + `FileProvider` + `FLAG_GRANT_READ_URI_PERMISSION` (permiso `REQUEST_INSTALL_PACKAGES`; la primera vez Android pide autorizar la fuente). Menú de ajustes: "Buscar actualizaciones", selector de canal estable/nightly, comprobación automática al abrir como máximo una vez al día (`SharedPreferences`), y "Actualizar yt-dlp" que llama a `YoutubeDL.getInstance().updateYoutubeDL(context)`.
3. **Firma.** El APK solo se actualiza si la firma coincide. Las builds locales de debug no se actualizarán desde releases; documentarlo en `android/README.md`.
4. **Nightly.** Workflow `nightly.yml` en push a `main`: compila el APK, lo firma con la misma clave, borra y recrea la prerelease `nightly` con tag movido al commit, versión `v<última>-nightly.<yyyymmdd>+<sha corto>`. Comprobar que `upgrade/version.go` ordena bien esa cadena contra la estable (`comparePrerelease` existe; añadir casos de test).

Criterio de salida: un APK instalado con versión antigua detecta la nueva, la descarga y el instalador del sistema la aplica conservando config y cookies.

### Fase 5. Release, CI y documentación

1. `release.yml`: job `android` que ejecuta `make android-core`, `./gradlew :app:assembleRelease` con el keystore de los secretos, renombra a `cliamp-x-<ver>-arm64-v8a.apk`, añade su SHA-256 al archivo de checksums y lo sube a la release junto a los paquetes Linux.
2. `ci.yml`: el job `android-core` de la fase 1 siempre; `./gradlew :app:assembleDebug` y `lint` solo cuando cambia `android/**` (filtro `paths`).
3. Docs (en inglés, salvo README): `docs/android.md` (instalar, iniciar sesión, permisos One UI, actualizar, limitaciones: sin kitty, sin MPRIS, música local solo con permiso), sección en `README.md`, filas nuevas en el mapa de `CLAUDE.md` (`android/`, `internal/extbin/`, `ui.render`), `docs/remote-control.md`. Recordar que `docs_links_test.go` y `config/docs_defaults_test.go` leen los docs: correr `make test` tras editar.
4. `android/README.md`: cómo compilar en local, variables de `local.properties`, cómo generar el keystore, cómo instalar con adb.

---

## 4. Verificación global antes de dar por terminado

- `mise exec -- make ci` y `make android-core` verdes.
- En el A55: instalar release firmada, iniciar sesión, reproducir 3 canciones seguidas de YouTube Music con gapless, pausar 10 minutos y reanudar sin reconexión (`YTDLResumable`), cambiar de tema, abrir EQ, radio automática (`R`), descargar una canción offline (`D`) y reproducirla en modo avión, rotar la pantalla y comprobar que la TUI cambia de tier sin cortarse, apagar la pantalla 30 minutos con audio, controles Bluetooth, actualización desde una versión anterior.
- Medir consumo: con pantalla apagada el proceso Go debe estar casi al 0% de CPU (`adb shell top`). Si no, `ui.render off` no está llegando.

## 5. Riesgos conocidos y qué hacer

| Riesgo | Señal | Acción |
|--------|-------|--------|
| oto/Oboe no abre audio desde el proceso hijo | `speaker.Init` falla dentro del APK | Comprobar que el proceso hijo hereda el UID de la app (lo hace). Si Oboe exige un `JavaVM`, cambiar oto a OpenSL ES (`driver_android.go` lo soporta por tag) antes de pensar en otra arquitectura |
| Google rechaza el login en WebView | Mensaje de navegador no seguro | Plan B de importar cookies.txt. Ya está en la fase 2 |
| youtubedl-android cambia su forma de lanzar Python | yt-dlp no arranca tras actualizar la librería | Las variables `CLIAMP_YTDLP*` aíslan el cambio al shell Kotlin |
| One UI mata el service | El audio para a los pocos minutos con pantalla apagada | Pantalla de batería de la fase 2, paso 10 |
| Repintado lento de la vista Termux con visualizadores densos | Lag al tocar | Bajar fps y filas de visualizador en la config por defecto del APK, no en el Go |
| 16 KB page size | El APK no instala o el binario no carga en kernels de 16 KB | El `max-page-size` de la fase 1, paso 6 |
| Tamaño del APK | ~45 MB por Python + ffmpeg | Aceptado. No optimizar en la primera versión |

## 6. Fuera de alcance de la primera versión

Widget de pantalla de inicio, Android Auto, Wear OS, otras ABIs, gráficos kitty, control del volumen del player con las teclas físicas, Play Store.

Diferido por el usuario en la UI nativa (`UI-SPEC.md` §10): el reproductor en horizontal, los visualizadores ClassicPeak y Mirror nativos (todo modo de barras se dibuja como Bars) y las etiquetas de TalkBack.
