# YouTube Music

cliamp-x reproduce YouTube Music con tu propia cuenta. El audio llega
directamente a través de [yt-dlp](https://github.com/yt-dlp/yt-dlp), así que no
hay anuncios.

## Requisitos

- `yt-dlp` en el `PATH`.
- `ffmpeg` para los formatos Opus y AAC que entrega YouTube.
- Un navegador con tu sesión de YouTube Music iniciada.

## Iniciar sesión

1. Abre [music.youtube.com](https://music.youtube.com) en tu navegador e inicia
   sesión una vez.
2. Ejecuta:

   ```sh
   cliamp-x login
   ```

   Para elegir un navegador concreto, usa la sintaxis de yt-dlp:

   ```sh
   cliamp-x login --browser brave-origin
   ```

cliamp-x copia las cookies de tu sesión a
`~/.config/cliamp-x/ytmusic_cookies.txt` con permisos `0600`. En cada inicio
las vuelve a leer del navegador, así que no tienes que iniciar sesión de nuevo.

Para cerrar la sesión y borrar las cookies guardadas:

```sh
cliamp-x logout
```

Sin sesión, la búsqueda y las páginas públicas siguen funcionando.

## Uso

Ejecuta `cliamp-x`. YouTube Music es el proveedor por defecto. Pulsa `Y` para
volver a él desde otro proveedor y `N` para abrir el navegador.

La pantalla principal (`[Cola]` en el título) muestra solo la cola: el álbum o
la playlist que suena. Para buscar música nueva, `Esc` abre **Explorar**
(`[Explorar]`), con los menús de abajo. Al elegir una canción en Explorar, su
lista reemplaza la cola y vuelves a la pantalla principal.

## Menús

El panel de YouTube Music tiene estas secciones:

| Sección | Filas | Contenido |
|---------|-------|-----------|
| Explorar | Álbumes | "Para ti": álbumes nuevos de artistas parecidos a los que escuchas, mezclados con los tuyos (♥ guardados, los de tus "me gusta" y los de tu inicio). La lista no termina: al bajar cargan más. Ordénala con `s` |
| Explorar | Artistas | Tus suscripciones (♥), los artistas de tu biblioteca y de tus "me gusta", mezclados con artistas relacionados que no conoces. Tampoco termina. Cada artista abre sus canciones populares, álbumes y singles |
| Explorar | Géneros | Los géneros de YouTube Music y, en "Más géneros", 44 más de nicho (shoegaze, city pop, vocaloid, post-punk, música de videojuegos...). Cada uno ofrece una mezcla distinta cada vez, sus canciones populares, la lista destacada o una lista de la comunidad |
| Explorar | Estados de ánimo | Relax, Energía, Concentración, Dormir... |
| Tu música | ♥ Favoritos | Tus "me gusta" de YouTube Music junto con todo lo que marcaste con `f`, también tus archivos locales |
| Tu música | Historial | Lo que escuchaste en cliamp-x (local y YouTube) y tu historial de la cuenta |
| Tu música | Música local | Todas las canciones de tu carpeta de música |
| Tu música | Tus playlists | Las playlists de tu biblioteca |
| Tus géneros | ★ Género | Los géneros que fijaste con `f` |
| Novedades de tus artistas | Lanzamientos | Los singles y álbumes de este año de los artistas a los que estás suscrito, los más recientes primero. Los que aún no abriste llevan `●`. Se actualiza en segundo plano cada 6 horas |
| Inicio | Mixes y estantes | Tus mixes, las selecciones rápidas y las playlists que recomienda tu inicio. Los álbumes y artistas del inicio están en Álbumes y Artistas |

Al iniciar, Inicio, Tus playlists y los géneros aparecen al instante con lo
que mostraban la última vez, guardado en `~/.local/share/cliamp-x/browse.json`.
cliamp-x los actualiza en segundo plano y la próxima vez que abras la sección
ya ves la versión nueva. `F5` o `Ctrl+R` en el panel fuerza la actualización.

### Descubrir música nueva

Álbumes (en el orden "Para ti") y Artistas se arman con tus gustos: parten de
los artistas que marcaste "me interesa", tus suscripciones, tus "me gusta" y lo
que escuchaste hace poco, abren sus artistas relacionados y traen sus álbumes.
Los relacionados sirven a su vez de punto de partida, así que la lista sigue
mientras bajas. `F5` (o `Ctrl+R`) empieza de nuevo con otros resultados; en
un género, arma otra mezcla.

### Me interesa y no me interesa

En la cola, en el panel Explorar y en el navegador:

- `I` (**me interesa**) en una canción, álbum, artista, género o lista: las
  recomendaciones traen más como eso. En una canción también es "me gusta"
  (♥) en tu cuenta.
- `H` (**no me interesa**): la fila desaparece y no vuelve en las
  recomendaciones, la radio automática ni las mezclas de género. En una
  canción también es "no me gusta" en tu cuenta, así YouTube Music aprende
  también; si suena, pasa a la siguiente. El menú de la fila (`O`) tiene
  además "No me interesa este artista".

Las calificaciones se guardan en `~/.config/cliamp-x/taste.json`. Ahí también
están las palabras y artistas que se ocultan siempre (`blocked_words` y
`blocked_artists`). Por defecto ocultan el reguetón, el urbano, el trap, la
bachata y la salsa: los géneros de YouTube Music con esos nombres, las listas
y álbumes que los nombran y una lista de artistas conocidos de esos géneros.
Edita el archivo con cliamp-x cerrado para cambiar las listas; una palabra
oculta solo cuando aparece completa ("trap" no oculta "trapped").

### Favoritos

`f` marca o desmarca según lo seleccionado:

- Una canción: "me gusta" en tu cuenta (y en Favoritos).
- Un álbum en Álbumes: lo guarda en tu biblioteca o lo quita.
- Un artista en Artistas: te suscribe o te desuscribe.
- Un género en Géneros: lo fija primero y en el panel.

### Música local

Los álbumes, artistas y géneros de tu carpeta de música (por defecto
`~/Music`, cambiable con `music_dir`) aparecen en los mismos menús, marcados
"· local". Un artista local con el mismo nombre que uno de YouTube Music se une
a él, así una fila abre los dos.

## Búsqueda

Pulsa `Ctrl+F` y escribe el nombre de una canción, un álbum o un artista. La
búsqueda también acepta links de YouTube y YouTube Music:

| Link | Resultado |
|------|-----------|
| `watch?v=...` o `youtu.be/...` | Una canción o un video |
| `playlist?list=...` | Una playlist |
| `browse/MPREb_...` | Un álbum |
| `channel/UC...` o `@nombre` | Un artista (sus canciones populares) |

`Ctrl+U` borra el texto del buscador.

## Vista previa

Si la terminal tiene filas de sobra, bajo la lista aparece la carátula de lo
que tienes seleccionado, con algunos datos:

- **Álbum:** tipo y año, artista, número de canciones y duración.
- **Artista:** oyentes mensuales, suscriptores y sus canciones más populares.
- **Playlist:** autor y duración.
- **Canción:** artista, álbum, año, duración y reproducciones.

Un video normal de YouTube (un remix no oficial, por ejemplo) se ve con su
miniatura en 16:9 o 4:3, sin recortar. Las imágenes de las filas cercanas se
descargan por adelantado y quedan guardadas en
`~/.local/share/cliamp-x/artwork`, así que al recorrer la lista aparecen sin
esperar. En kitty y Ghostty la carátula se ve como imagen real (protocolo
gráfico de kitty); en otras terminales, o dentro de tmux, se dibuja con
bloques de colores, con menos detalle. `preview_images = "blocks"` fuerza los
bloques y `hide_preview = true` en `config.toml` oculta la vista previa.

## Más como esto y buscador de remixes

- `M` (**más como esto**) arma una radio con canciones parecidas a la fila
  seleccionada: una canción, un álbum, un artista o una playlist (de un álbum
  o artista toma su primera canción). Como "Iniciar radio" en YouTube Music,
  reemplaza la cola y empieza a sonar. Sin nada seleccionado usa la canción
  que suena.
- `X` busca **remixes y otras versiones** de la canción seleccionada o de la
  que suena, y las agrupa en Remixes, Slowed + reverb, Sped up, Nightcore,
  Bootlegs y mashups, y Versiones y covers. Descarta lo que no menciona el
  título original o dura demasiado distinto; en un título de una sola palabra
  pide además el nombre del artista.

Ambas funcionan en el panel de YouTube Music, en el navegador de álbumes y
artistas, en la búsqueda y en la cola.

## Pausa

Mientras suena una canción, cliamp-x la descarga entera en memoria (unos
pocos MB). Así, quitar la pausa la reanuda al instante aunque haya pasado
mucho tiempo, sin volver a conectarse a YouTube.

## Descargas y letras

- `D` descarga para escuchar sin conexión la canción, el álbum o la playlist
  seleccionada (de un artista, sus canciones populares). Se guardan en
  `~/Music/cliamp-x/Artista/Álbum/` con su carátula y etiquetas, sin
  recomprimir el audio. Una canción descargada suena desde el archivo, aunque
  no haya conexión, y la vista previa la marca con `↓`. Ver
  [Configuración](configuration.md#offline-downloads).
- Las letras de YouTube Music se suman a las de LRCLIB y NetEase. Como YouTube
  Music no trae tiempos, las letras sincronizadas de LRCLIB tienen prioridad y
  las de YouTube Music se usan cuando no hay otras. Las letras encontradas se
  guardan en `~/.local/share/cliamp-x/lyrics/`, así que una canción repetida
  muestra su letra al instante, también sin conexión.

## SponsorBlock

Muchos videos musicales de YouTube traen una intro hablada, un final con
créditos o un patrocinio. cliamp-x los salta solo, usando las marcas de la
comunidad de [SponsorBlock](https://sponsor.ajay.app): la intro se salta
empezando la canción después de ella, y un final que llega hasta el término
de la canción pasa directo a la siguiente. La barra de progreso muestra las
partes marcadas en otro color y la barra de estado avisa cada salto. Se
configura en la sección `[sponsorblock]` de `config.toml` (ver
[Configuración](configuration.md#sponsorblock)).

## Radio automática

Con la radio automática activa, cuando la cola termina cliamp-x sigue con
canciones parecidas a la última. Está desactivada por defecto: un álbum o una
playlist se detiene al terminar. Pulsa `R` para activarla o desactivarla (se
guarda como `auto_radio` en la sección `[ytmusic]`). Las canciones que marcaste
"no me interesa", y los géneros y artistas bloqueados, nunca entran.

## Historial de YouTube Music

Con `report_history = true`, cliamp-x agrega las canciones que escuchas a tu
historial de YouTube Music, lo que mejora tus recomendaciones. Está desactivado
por defecto.

## Configuración

Todas las claves de `[ytmusic]` son opcionales:

```toml
[ytmusic]
cookies_from = "brave-origin"   # navegador en sintaxis de yt-dlp; vacío = detectar
expand_playlist = true          # los links list= cargan la playlist completa
language = "es"                 # idioma de las páginas (por defecto, desde $LANG)
region = "CL"                   # país de las páginas (por defecto, desde $LANG)
auto_radio = false              # seguir con canciones parecidas al final de la cola
report_history = false          # agregar lo que escuchas a tu historial
```

Pon `enabled = false` para desactivar el proveedor. Consulta
[configuration.md](configuration.md#youtube-music) para el resto de opciones.

## Solución de problemas

- **No encuentra la sesión**: abre music.youtube.com en el navegador, comprueba
  que la sesión está iniciada y vuelve a ejecutar `cliamp-x login`, indicando el
  navegador con `--browser` si tienes varios.
- **No suena nada**: comprueba que `yt-dlp` y `ffmpeg` están instalados y
  actualiza yt-dlp (`yt-dlp -U` o el gestor de paquetes de tu sistema).
- **Una canción no arranca**: el aviso dice qué hacer cuando reconoce la
  causa: sin conexión, sesión caducada (`cliamp-x login`), yt-dlp
  desactualizado (`yt-dlp -U`) o video no disponible en tu país.
- **Errores**: revisa `~/.config/cliamp-x/cliamp.log`, o arranca con
  `cliamp-x --log-level debug`.
