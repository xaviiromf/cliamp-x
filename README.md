# cliamp-x

Un reproductor de música retro para la terminal, inspirado en Winamp, centrado
en **YouTube Music** y en tus **archivos locales**. Funciona en Linux.

cliamp-x es un fork de [cliamp](https://github.com/bjarneo/cliamp), creado por
Bjarne Øverli. Todo el mérito del reproductor original es suyo. cliamp-x quita
los demás proveedores y se concentra en YouTube Music. Se distribuye bajo la
licencia MIT, igual que el original (ver [LICENSE](LICENSE)).

## Funciones

- **YouTube Music sin anuncios con tu cuenta**: el audio llega directamente a
  través de yt-dlp.
- **Menús personalizados**: Inicio, Favoritos, Álbumes, Artistas, Géneros,
  Playlists e Historial.
- **Búsqueda por nombre o por link** de YouTube y YouTube Music (canciones,
  playlists, álbumes y artistas).
- **Archivos locales integrados en los mismos menús**, junto con tus playlists
  locales, favoritos e historial.
- **Ecualizador** de 10 bandas con presets por género.
- **Visualizadores** del espectro, con muchos modos.
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

### Script de instalación

```sh
git clone https://github.com/xaviiromf/cliamp-x.git
cd cliamp-x
./scripts/install.sh
```

### Compilar desde el código fuente

Necesitas [Go](https://go.dev/dl/) (la versión de `go.mod`) y las cabeceras de
ALSA, FLAC, Vorbis, Ogg y mpg123:

```sh
# Debian/Ubuntu
sudo apt install libasound2-dev libflac-dev libvorbis-dev libogg-dev libmpg123-dev

# Arch
sudo pacman -S alsa-lib flac libvorbis libogg mpg123

# Fedora
sudo dnf install alsa-lib-devel flac-devel libvorbis-devel libogg-devel mpg123-devel
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
| `f` | Marcar o quitar de favoritos |
| `e` | Cambiar el preset del ecualizador |
| `v` | Cambiar de visualizador |
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
