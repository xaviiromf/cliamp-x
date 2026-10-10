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

## Menús

El panel de YouTube Music tiene tres secciones:

| Sección | Filas | Contenido |
|---------|-------|-----------|
| Explorar | Álbumes | Los álbumes que guardaste (♥) primero, después los de tus canciones con "me gusta" y los que recomienda tu inicio. Ordénalos con `s` |
| Explorar | Artistas | Tus suscripciones (♥), los artistas de tu biblioteca y los de tus "me gusta". Cada artista abre sus canciones populares, álbumes y singles |
| Explorar | Géneros | Los géneros de YouTube Music. Cada uno ofrece una mezcla, sus canciones populares, la lista destacada o una lista de la comunidad |
| Explorar | Estados de ánimo | Relax, Energía, Concentración, Dormir... |
| Tu música | ♥ Favoritos | Tus "me gusta" de YouTube Music junto con todo lo que marcaste con `f`, también tus archivos locales |
| Tu música | Historial | Lo que escuchaste en cliamp-x (local y YouTube) y tu historial de la cuenta |
| Tu música | Música local | Todas las canciones de tu carpeta de música |
| Tu música | Tus playlists | Las playlists de tu biblioteca |
| Tus géneros | ★ Género | Los géneros que fijaste con `f` |
| Inicio | Mixes y estantes | Selecciones rápidas, tus mixes, lanzamientos y recomendaciones |

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

## Radio automática

Cuando la cola termina, cliamp-x sigue con canciones parecidas a la última.
Pulsa `R` para activarla o desactivarla (se guarda como `auto_radio` en la
sección `[ytmusic]`).

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
auto_radio = true               # seguir con canciones parecidas al final de la cola
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
- **Errores**: revisa `~/.config/cliamp-x/cliamp.log`, o arranca con
  `cliamp-x --log-level debug`.
