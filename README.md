# cliamp-x

Un reproductor de música retro para la terminal, inspirado en Winamp, centrado
en **YouTube Music** y en tus **archivos locales**. Funciona en Linux.

cliamp-x es un fork de [cliamp](https://github.com/bjarneo/cliamp), creado por
Bjarne Øverli. Todo el mérito del reproductor original es suyo. cliamp-x quita
los demás proveedores y se concentra en YouTube Music. Se distribuye bajo la
licencia MIT, igual que el original (ver [LICENSE](LICENSE)).

## Funciones

- **YouTube Music sin anuncios con tu cuenta**: el audio llega directamente a
  través de yt-dlp, con la sesión de tu navegador (Premium incluido).
- **Inicio de sesión persistente**: `cliamp-x login` copia la sesión de tu
  navegador una vez (Brave, Brave Origin, Chrome, Chromium, Edge, Vivaldi,
  Opera, Firefox, LibreWolf, Zen...) y cliamp-x la renueva en cada inicio.
- **Menús personalizados**: Explorar (Álbumes, Artistas, Géneros, Estados de
  ánimo), Tu música (♥ Favoritos, Historial, Música local y tus playlists) e
  Inicio (tus mixes, selecciones rápidas y recomendaciones).
- **Todo ordenado según tus gustos**: los álbumes que guardaste y los artistas
  a los que te suscribiste van primero con ♥, seguidos de los que más
  aparecen en tus "me gusta". `f` guarda un álbum o te suscribe a un artista
  en tu cuenta, y en una canción le da (o quita) "me gusta".
- **Géneros favoritos**: marca un género con `f` y queda primero y fijado en
  el panel.
- **Búsqueda por nombre o por link** de YouTube y YouTube Music (`watch?v=`,
  `youtu.be/`, shorts, playlists, álbumes, canales y `@artista`).
- **Radio automática**: cuando la cola termina, sigue con canciones parecidas
  a la última (`R` la activa o desactiva).
- **Archivos locales integrados en los mismos menús**: tus álbumes, artistas y
  géneros locales aparecen junto a los de YouTube Music (marcados "· local"), y
  Favoritos e Historial mezclan ambos.
- **Volumen parejo** (`n`): normaliza el volumen de cada canción (o de cada
  álbum) para que un remix fuerte no salte después de una canción suave. Usa
  las etiquetas ReplayGain de tus archivos y mide el resto mientras suena.
- **Crossfade** opcional entre canciones (`crossfade = 6`), sin cortar los
  álbumes continuos.
- **SponsorBlock**: salta intros, finales, patrocinios y partes sin música de
  los videos de YouTube, según las marcas de la comunidad.
- **Ecualizador visual** (`E`): curva de 10 bandas agrupadas en graves, medios y
  agudos, presets por género (Rock, Metal, Pop, Electronic, Hip-Hop,
  Reggaeton, Cumbia, Jazz, Classical, Lo-Fi, Soundtrack...), vista previa al
  recorrerlos, comparación A/B, preamplificador automático contra la
  saturación y modo **EQ automático** que elige el preset según el género de
  cada canción.
- **Visualizadores** del espectro: más de 40 modos (`v` los recorre), entre
  ellos Spectrogram, Radial, Starfield, Plasma, Tunnel, Helix, Ripple, Aurora,
  Skyline y Spiral. Se ven en todas las pantallas, también al buscar y al
  explorar, ajustados al espacio libre, y en pantalla completa (`V`).
- **Vista previa con carátula**: al recorrer álbumes, artistas, playlists,
  canciones o resultados de búsqueda, abajo aparece la carátula (o la miniatura
  del video, con su forma 16:9 o 4:3) con datos como el artista, el año, la
  duración, las canciones, los oyentes o las reproducciones. Las imágenes de
  las filas cercanas se cargan por adelantado, así que aparecen sin esperar.
- **Letras** sincronizadas.
- **MPRIS**: controla la reproducción con las teclas multimedia, `playerctl` o
  el panel de tu escritorio.
- **Control remoto** desde otra terminal, scripts o barras de estado, y un modo
  sin interfaz (`cliamp-x --daemon`).
- Temas, plugins en Lua, streaming por SSH y URLs de streams.

## Requisitos

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) para YouTube Music.
- [ffmpeg](https://ffmpeg.org/) para AAC, ALAC, Opus y WMA (YouTube entrega
  Opus y AAC).
- ALSA. Si usas PipeWire o PulseAudio, instala el puente `pipewire-alsa` o
  `pulseaudio-alsa`. Ver [Solución de problemas](#solución-de-problemas).

## Instalación

### Script de instalación (cualquier distribución)

```sh
git clone https://github.com/xaviiromf/cliamp-x.git
cd cliamp-x
./scripts/install.sh
```

El script detecta tu gestor de paquetes (apt, dnf/yum, zypper, pacman, apk,
xbps, emerge), instala yt-dlp, ffmpeg, ALSA y el puente de PipeWire si lo usas,
compila cliamp-x (descarga Go si el de tu distribución es antiguo) y lo instala
en `~/.local/bin` con su acceso directo. Se prueba en cada cambio sobre Ubuntu,
Debian, Fedora, Arch, openSUSE, Alpine y Void.

- `./scripts/install.sh --release` instala el binario publicado en vez de
  compilarlo (con `gh` autenticado si el repositorio es privado).
- `./scripts/install.sh --uninstall` lo desinstala (tu configuración se queda en
  `~/.config/cliamp-x`).
- Otras opciones: `--prefix DIR`, `--no-deps`, `--yes`.

### Paquetes

Cada release publica binarios para x86_64 y aarch64 (compatibles con glibc 2.28
o superior), un `.tar.gz`, un `.deb` y un `.rpm`. En Arch hay un PKGBUILD en
[packaging/aur](packaging/aur/PKGBUILD) (`makepkg -si`).

### Compilar a mano

Necesitas [Go](https://go.dev/dl/) 1.21 o superior (descarga solo la versión
de `go.mod`), gcc y las cabeceras de ALSA:

```sh
# Debian/Ubuntu
sudo apt install golang-go gcc make libasound2-dev

# Arch
sudo pacman -S go gcc make alsa-lib

# Fedora
sudo dnf install golang gcc make alsa-lib-devel
```

Luego compila:

```sh
make build      # crea ./cliamp-x
make install    # opcional: lo copia a ~/.local/bin/
```

## Primer uso

1. Abre [music.youtube.com](https://music.youtube.com) en tu navegador e inicia
   sesión.
2. Copia la sesión a cliamp-x:

   ```sh
   cliamp-x login
   ```

3. Arranca el reproductor:

   ```sh
   cliamp-x
   ```

cliamp-x renueva las cookies desde el navegador en cada inicio, así que no
tienes que volver a iniciar sesión. Ver [docs/youtube-music.md](docs/youtube-music.md).

También puedes reproducir archivos y URLs directamente:

```sh
cliamp-x ~/Music                     # una carpeta
cliamp-x *.mp3 *.flac                # archivos
cliamp-x https://example.com/stream  # una URL
```

## Teclas principales

| Tecla | Acción |
|---|---|
| `Space` | Reproducir / pausar |
| `>` `.` / `<` `,` | Pista siguiente / anterior |
| `Left` `Right` | Retroceder / avanzar 5 s |
| `+` `-` | Subir / bajar volumen |
| `Ctrl+F` | Buscar en YouTube Music |
| `N` | Abrir el navegador del proveedor |
| `Y` / `L` | Ir a YouTube Music / a Local |
| `a` | Añadir a la cola (reproducir a continuación) |
| `f` | Favorito: "me gusta" en una canción, guardar un álbum, suscribirse a un artista, fijar un género |
| `e` | Cambiar el preset del ecualizador |
| `E` | Abrir el panel del ecualizador |
| `R` | Activar o desactivar la radio automática |
| `v` | Cambiar de visualizador |
| `}` / `{` | Subir / bajar la sensibilidad del visualizador |
| `n` | Normalización de volumen: desactivada, por canción, por álbum |
| `y` | Mostrar las letras |
| `z` / `r` | Aleatorio / modo de repetición |
| `Ctrl+K` / `?` | Ver todas las teclas |
| `q` | Salir |

La lista completa está en [docs/keybindings.md](docs/keybindings.md).

## Control remoto

Controla un cliamp-x en marcha desde un script, una barra de estado o un atajo
de teclado:

```sh
cliamp-x toggle
cliamp-x next
cliamp-x status --json
```

Ejecuta `cliamp-x --daemon` para reproducir sin interfaz. Ver
[docs/headless.md](docs/headless.md) y [docs/remote-control.md](docs/remote-control.md).

## Documentación

- [YouTube Music](docs/youtube-music.md)
- [Teclas](docs/keybindings.md)
- [Configuración](docs/configuration.md)
- [Opciones de línea de comandos](docs/cli.md)
- [Playlists locales y favoritos](docs/playlists.md)
- [Historial](docs/history.md)
- [Streaming de URLs](docs/streaming.md)
- [yt-dlp](docs/yt-dlp.md)
- [Streaming por SSH](docs/ssh-streaming.md)
- [Calidad de audio](docs/audio-quality.md)
- [Letras](docs/lyrics.md)
- [Temas](docs/themes.md)
- [Plugins](docs/plugins.md) y [plugins de la comunidad](docs/community-plugins.md)
- [Control remoto (IPC)](docs/remote-control.md)
- [Modo sin interfaz](docs/headless.md)
- [Controles multimedia (MPRIS)](docs/mediactl.md)
- [Links cliamp-x://](docs/url-scheme.md)

## Solución de problemas

**No hay salida de audio**

cliamp-x muestra `audio output unavailable` cuando no puede abrir un
dispositivo de salida. En sistemas Linux con PipeWire o PulseAudio, el backend
ALSA de cliamp-x necesita un paquete puente para enviar el audio al servidor de
sonido:

- **PipeWire:** `pipewire-alsa`
- **PulseAudio:** `pulseaudio-alsa` (`libasound2-plugins` en Debian/Ubuntu)

Instala el paquete de tu sistema:

```sh
# PipeWire (Arch)
sudo pacman -S pipewire-alsa

# PulseAudio (Arch)
sudo pacman -S pulseaudio-alsa

# Debian/Ubuntu (PipeWire)
sudo apt install pipewire-alsa

# Debian/Ubuntu (PulseAudio, incluido WSL2)
sudo apt install libasound2-plugins
```

En WSL2, consulta [WSL2](docs/configuration.md#wsl2-windows-subsystem-for-linux)
para el paso extra de configuración de ALSA.

## Créditos

cliamp-x se basa en [cliamp](https://github.com/bjarneo/cliamp) de Bjarne
Øverli ([x.com/iamdothash](https://x.com/iamdothash)). Usa
[Bubbletea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss) y
[Beep](https://github.com/gopxl/beep).

## Aviso

Usa este software bajo tu propia responsabilidad. Los autores no se hacen
responsables de daños o problemas derivados de su uso. Respeta los términos de
servicio de YouTube.
