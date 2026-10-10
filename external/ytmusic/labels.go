package ytmusic

import "strings"

// labels are the provider's own UI words, in Spanish or English after the
// YouTube Music language.
type labels struct {
	explore, yourMusic, home, genresSection            string
	favorites, history, albums, artists, genres, moods string
	topSongs, singles                                  string
	sortTop, sortAlpha, sortArtist, sortYear           string
	genreMix, genreSongs, genreFeatured                string
	genreCommunity, genreLabel, moodLabel              string
	searchSongs, searchAlbums, searchArtists           string
	searchPlaylists, searchLink, localMusic, localTag  string
	subscribers, songsWord                             string
	newReleases                                        string
}

var spanish = labels{
	explore: "Explorar", yourMusic: "Tu música", home: "Inicio", genresSection: "Tus géneros",
	favorites: "♥ Favoritos", history: "Historial", albums: "Álbumes", artists: "Artistas",
	genres: "Géneros", moods: "Estados de ánimo",
	topSongs: "Canciones populares", singles: "Singles y EP",
	sortTop: "Tus favoritos primero", sortAlpha: "A-Z", sortArtist: "Por artista", sortYear: "Más recientes",
	genreMix: "Mezcla del género", genreSongs: "Canciones populares", genreFeatured: "Lista destacada",
	genreCommunity: "Lista de la comunidad", genreLabel: "Géneros", moodLabel: "Estados de ánimo",
	searchSongs: "Canciones", searchAlbums: "Álbumes", searchArtists: "Artistas", searchPlaylists: "Listas", searchLink: "Desde el enlace",
	localMusic: "Música local", localTag: " · local", subscribers: "suscriptores", songsWord: "canciones",
	newReleases: "Novedades de tus artistas",
}

var english = labels{
	explore: "Explore", yourMusic: "Your music", home: "Home", genresSection: "Your genres",
	favorites: "♥ Favorites", history: "History", albums: "Albums", artists: "Artists",
	genres: "Genres", moods: "Moods",
	topSongs: "Top songs", singles: "Singles & EPs",
	sortTop: "Your favorites first", sortAlpha: "A-Z", sortArtist: "By artist", sortYear: "Newest",
	genreMix: "Genre mix", genreSongs: "Popular songs", genreFeatured: "Featured playlist",
	genreCommunity: "Community playlist", genreLabel: "Genres", moodLabel: "Moods",
	searchSongs: "Songs", searchAlbums: "Albums", searchArtists: "Artists", searchPlaylists: "Playlists", searchLink: "From the link",
	localMusic: "Local music", localTag: " · local", subscribers: "subscribers", songsWord: "songs",
	newReleases: "New from your artists",
}

func labelsFor(hl string) labels {
	if strings.HasPrefix(strings.ToLower(hl), "es") {
		return spanish
	}
	return english
}
