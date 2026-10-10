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

El navegador de YouTube Music ofrece estos menús:

| Menú | Contenido |
|------|-----------|
| Inicio | Las recomendaciones de tu página de inicio |
| Favoritos | Las canciones que te gustan |
| Álbumes | Los álbumes de tu biblioteca |
| Artistas | Los artistas que sigues |
| Géneros | Los géneros y estados de ánimo de YouTube Music |
| Playlists | Tus playlists y las que guardaste |
| Historial | Lo que escuchaste hace poco |

Tus playlists locales y tus archivos aparecen en los mismos menús.

## Búsqueda

Pulsa `Ctrl+F` y escribe el nombre de una canción, un álbum o un artista. La
búsqueda también acepta links de YouTube y YouTube Music:

| Link | Resultado |
|------|-----------|
| `watch?v=...` o `youtu.be/...` | Una canción o un video |
| `playlist?list=...` | Una playlist |
| `browse/MPREb_...` | Un álbum |
| `channel/UC...` | Un artista |

## Radio automática

Cuando la cola termina, cliamp-x sigue con canciones parecidas a la última.
Para desactivarlo, pon `auto_radio = false` en la sección `[ytmusic]`.

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
