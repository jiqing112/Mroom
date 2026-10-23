package store

// Song is the canonical track record shared by the web API and Subsonic layer.
type Song struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	ArtistID    string  `json:"artistId"`
	Artist      string  `json:"artist"`
	AlbumID     string  `json:"albumId"`
	Album       string  `json:"album"`
	AlbumArtist string  `json:"albumArtist"`
	Genre       string  `json:"genre"`
	Track       int     `json:"track"`
	Disc        int     `json:"disc"`
	Year        int     `json:"year"`
	Duration    float64 `json:"duration"`
	Bitrate     int     `json:"bitrate"`
	Size        int64   `json:"size"`
	Suffix      string  `json:"suffix"`
	ContentType string  `json:"contentType"`
	CoverArt    string  `json:"coverArt"`
	Path        string  `json:"-"`
	Created     int64   `json:"created"`
	Starred     bool    `json:"starred"`
	StarredAt   int64   `json:"-"`
	Rating      int     `json:"rating"`
	PlayCount   int     `json:"playCount"`
	HasLyrics   bool    `json:"hasLyrics"`
}

// Album groups songs by (album artist, album name).
type Album struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ArtistID  string `json:"artistId"`
	Artist    string `json:"artist"`
	Year      int    `json:"year"`
	Genre     string `json:"genre"`
	CoverArt  string `json:"coverArt"`
	SongCount int    `json:"songCount"`
	Duration  int    `json:"duration"`
	Created   int64  `json:"created"`
	Starred   bool   `json:"starred"`
	StarredAt int64  `json:"-"`
	Rating    int    `json:"rating"`
	Songs     []Song `json:"songs,omitempty"`
}

// Artist is a performer entry in the library index.
type Artist struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	AlbumCount int     `json:"albumCount"`
	SongCount  int     `json:"songCount"`
	CoverArt   string  `json:"coverArt"`
	Starred    bool    `json:"starred"`
	StarredAt  int64   `json:"-"`
	Albums     []Album `json:"albums,omitempty"`
}

type Playlist struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Comment   string `json:"comment"`
	Owner     string `json:"owner"`
	OwnerID   string `json:"ownerId"`
	Public    bool   `json:"public"`
	SongCount int    `json:"songCount"`
	Duration  int    `json:"duration"`
	Created   int64  `json:"created"`
	Changed   int64  `json:"changed"`
	CoverArt  string `json:"coverArt"`
	Entries   []Song `json:"entries,omitempty"`
}

type Genre struct {
	Name       string `json:"name"`
	SongCount  int    `json:"songCount"`
	AlbumCount int    `json:"albumCount"`
}

type User struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	IsAdmin        bool   `json:"isAdmin"`
	Guest          bool   `json:"guest,omitempty"`
	APIKey         string `json:"apiKey,omitempty"`
	Created        int64  `json:"created"`
	SessionVersion int64  `json:"-"`
}

// TrackInput carries everything the scanner learned about one file.
type TrackInput struct {
	RelPath     string
	AbsPath     string
	Size        int64
	ModTime     int64
	Title       string
	Artist      string
	AlbumArtist string
	Album       string
	Genre       string
	Track       int
	Disc        int
	Year        int
	Duration    float64
	Bitrate     int
	SampleRate  int
	Channels    int
	Suffix      string
	ContentType string
	CoverData   []byte
	CoverMime   string
	Lyrics      string
}

type SongFilter struct {
	Search      string
	AlbumID     string
	ArtistID    string
	Genre       string
	StarredOnly bool
	Sort        string
	Limit       int
	Offset      int
}
