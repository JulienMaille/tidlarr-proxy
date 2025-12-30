package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type Album struct {
	Artist       string
	Title        string
	Edition      string
	ReleaseDate  string
	Publisher    string
	CoverUrl     string
	SamplingRate int64
	BitDepth     int64
	Id           string
	NumTracks    int64
	Channels     int64
	Duration     int64
	Size         int64
}

func handleIndexerRequest(w http.ResponseWriter, r *http.Request) {
	var queryApiKey string = r.URL.Query().Get("apikey")
	if queryApiKey != ApiKey {
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
		<error code="100" description="Incorrect user credentials"/>`))
		return
	}
	switch query := r.URL.Query().Get("t"); query {
	case "caps":
		caps(w, *r.URL)
	case "music":
		music(w, *r.URL)
	case "search":
		search(w, *r.URL)
	case "fakenzb":
		fakenzb(w, *r.URL)
	default:
		fmt.Println("Indexer unknown request:")
		fmt.Println(r.Method)
		fmt.Println(r.URL.String())
		fmt.Println(r.Header)
		buffer := make([]byte, 100)
		for {
			n, err := r.Body.Read(buffer)
			fmt.Printf("%q\n", buffer[:n])
			if err == io.EOF {
				break
			}
		}
		w.Write([]byte("Request received!"))
	}
}

func caps(w http.ResponseWriter, u url.URL) {
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<caps>
    <limits max="5000" default="5000"/>
    <registration available="no" open="no"/>
    <searching>
        <search available="yes" supportedParams="q"/>
        <tv-search available="no" supportedParams=""/>
        <movie-search available="no" supportedParams=""/>
        <audio-search available="no" supportedParams=""/>
        <music-search available="no" supportedParams=""/>
    </searching>
    <categories>
        <category id="3000" name="Audio">
            <subcat id="3010" name="Audio/MP3"/>
            <subcat id="3020" name="Audio/Video"/>
            <subcat id="3030" name="Audio/Audiobook"/>
            <subcat id="3040" name="Audio/Lossless"/>
            <subcat id="3050" name="Audio/Podcast"/>
        </category>
    </categories>
</caps>
	`))
}

type Rss struct {
	XMLName string  `xml:"rss"`
	Version string  `xml:"version,attr"`
	Newznab string  `xml:"xmlns:newznab,attr"`
	Channel Channel `xml:"channel"`
}

type Channel struct {
	Title           string          `xml:"title"`
	Description     string          `xml:"description"`
	NewznabResponse NewznabResponse `xml:"newznab:response"`
	Items           []Item          `xml:"item"`
}

type NewznabResponse struct {
	Offset int `xml:"offset,attr"`
	Total  int `xml:"total,attr"`
}

type Item struct {
	Title       string        `xml:"title"`
	Guid        Guid          `xml:"guid"`
	Link        string        `xml:"link"`
	Comments    string        `xml:"comments"`
	PubDate     string        `xml:"pubDate"`
	Category    string        `xml:"category"`
	Description string        `xml:"description"`
	Enclosure   Enclosure     `xml:"enclosure"`
	Attrs       []NewznabAttr `xml:"newznab:attr"`
}

type Guid struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type Enclosure struct {
	Url    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type NewznabAttr struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func music(w http.ResponseWriter, u url.URL) {
	if u.Query().Get("q") == "" && u.Query().Get("artist") == "" && u.Query().Get("album") == "" {
		fmt.Println("searching with no query, responding garbage...")
		rss := Rss{
			Version: "2.0",
			Newznab: "http://www.newznab.com/DTD/2010/feeds/attributes/",
			Channel: Channel{
				Title:       "example.com",
				Description: "example.com API results",
				NewznabResponse: NewznabResponse{
					Offset: 0,
					Total:  1234,
				},
				Items: []Item{
					{
						Title: "A.Public.Domain.Album.Name",
						Guid: Guid{
							IsPermaLink: true,
							Value:       "http://servername.com/rss/viewnzb/e9c515e02346086e3a477a5436d7bc8c",
						},
						Link:        "http://servername.com/rss/nzb/e9c515e02346086e3a477a5436d7bc8c&i=1&r=18cf9f0a736041465e3bd521d00a90b9",
						Comments:    "http://servername.com/rss/viewnzb/e9c515e02346086e3a477a5436d7bc8c#comments",
						PubDate:     "Sun, 06 Jun 2010 17:29:23 +0100",
						Category:    "Music > MP3",
						Description: "Some music",
						Enclosure: Enclosure{
							Url:    "http://servername.com/rss/nzb/e9c515e02346086e3a477a5436d7bc8c&i=1&r=18cf9f0a736041465e3bd521d00a90b9",
							Length: 154653309,
							Type:   "application/x-nzb",
						},
						Attrs: []NewznabAttr{
							{Name: "category", Value: "3000"},
							{Name: "category", Value: "3010"},
							{Name: "size", Value: "144967295"},
							{Name: "artist", Value: "Bob Smith"},
							{Name: "album", Value: "Groovy Tunes"},
							{Name: "publisher", Value: "Epic Music"},
							{Name: "year", Value: "2011"},
							{Name: "tracks", Value: "track one|track two|track three"},
							{Name: "coverurl", Value: "http://servername.com/covers/music/12345.jpg"},
							{Name: "review", Value: "This album is great"},
						},
					},
				},
			},
		}
		w.Write([]byte(xml.Header))
		xml.NewEncoder(w).Encode(rss)
		return
	}
	var queryUrl string = "/search/?al=" + url.QueryEscape(u.Query().Get("artist")) + "+" + url.QueryEscape(u.Query().Get("album"))
	queryUrl = strings.Replace(queryUrl, " ", "+", -1)
	
	respondWithSearch(w, queryUrl)
}

func search(w http.ResponseWriter, u url.URL) {
	//doing the actual querying request
	//getting the query parameters
	var query string = url.QueryEscape(u.Query().Get("q"))
	//Searching with no query, probably Prowlarr testing the indexer. Returning same garbage as with t=music
	if query == "" {
		music(w, u)
		return
	}
	//Tidal API (sachinsenal0x64/hifi) doesn't support setting limit or offset as of right now. Just use the first and only 25 results
	var queryUrl string = "/search/?al=" + query
	respondWithSearch(w, queryUrl)
}

func respondWithSearch(w http.ResponseWriter, queryUrl string) {
	rss, err := buildSearchResponse(queryUrl)
	if err != nil {
		// Log error, maybe return empty RSS?
		fmt.Println("Error building search response:", err)
		return
	}
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(rss)
}

func releaseName(album Album) (name string) {
	release := album.ReleaseDate[0:4]
	if QualityId == "HIGH" {
		name = album.Artist + "-" + album.Title + "-WEB-320-AAC-" + release + "-TIDLARR"
	} else {
		name = album.Artist + "-" + album.Title + "-" + strconv.FormatInt(album.BitDepth, 10) + "BIT-" + strconv.FormatInt(album.SamplingRate, 10) + "-KHZ-WEB-FLAC-" + release + "-TIDLARR"
	}
	return name
}

func buildSearchResponse(queryUrl string) (*Rss, error) {
	bodyBytes, err := request(queryUrl)
	if err != nil {
		return nil, err
	}
	var Albums []Album
	//iterate over each album and create an Album struct object from it
	result := gjson.Get(bodyBytes, "data.albums.items")
	result.ForEach(func(key, value gjson.Result) bool {
		var album Album
		var resultString string = value.String()
		album.Artist = gjson.Get(resultString, "artists.0.name").String()
		album.Title = gjson.Get(resultString, "title").String()
		album.Edition = gjson.Get(resultString, "version").String()
		album.ReleaseDate = gjson.Get(resultString, "releaseDate").String()
		album.Publisher = gjson.Get(resultString, "copyright").String()
		album.Id = gjson.Get(resultString, "id").String()
		album.NumTracks = gjson.Get(resultString, "numberOfTracks").Int()
		//Assuming Stereo, 16 bit and 44.1KHz because checking this would take a lot more api calls
		//Also skipping cover art url because we can just grab that later
		album.Channels = 2
		album.SamplingRate = 44
		album.BitDepth = 16
		album.Duration = gjson.Get(resultString, "duration").Int()
		
		if QualityId == "HIGH" {
			// AAC 320kbps estimate
			album.Size = int64(320 * 1000 * album.Duration / 8)
		} else {
			// FLAC (default)
			album.Size = int64(float64(((album.SamplingRate * 1000) * (album.BitDepth * album.Channels * album.Duration) / 8)) * 0.7)
		}
		
		Albums = append(Albums, album)

		return true // keep iterating
	})

	items := []Item{}
	for _, album := range Albums {
		// Removed regex sanitization of album.Title and album.Artist
		
		timestamp, _ := time.Parse("2006-01-02", album.ReleaseDate)
		Release := releaseName(album)

		var categoryName string
		var categoryAttrs []NewznabAttr

		if QualityId == "HIGH" {
			// AAC 320
			categoryName = "Audio > MP3"
			categoryAttrs = []NewznabAttr{
				{Name: "category", Value: "3000"},
				{Name: "category", Value: "3010"},
				{Name: "size", Value: strconv.FormatInt(album.Size, 10)},
			}
		} else {
			// FLAC / Lossless (default)
			categoryName = "Audio > Lossless"
			categoryAttrs = []NewznabAttr{
				{Name: "category", Value: "3000"},
				{Name: "category", Value: "3040"},
				{Name: "size", Value: strconv.FormatInt(album.Size, 10)},
			}
		}

		items = append(items, Item{
			Title: Release,
			Guid: Guid{IsPermaLink: true, Value: "http://www.tidal.com/album?id=" + album.Id},
			Link: "http://www.tidal.com/album/" + album.Id,
			Comments: "http://www.tidal.com/album/" + album.Id + "#comments",
			PubDate: timestamp.Format("Mon, 02 Jan 2006 15:04:05 -0700"),
			Category: categoryName,
			Description: album.Artist + " " + album.Title,
			Enclosure: Enclosure{
				Url: "/indexer?t=fakenzb&name=" + url.QueryEscape(Release) + "&tidalid=" + album.Id + "&numtracks=" + strconv.FormatInt(album.NumTracks, 10) + "&apikey=" + ApiKey,
				Type: "application/x-nzb",
			},
			Attrs: categoryAttrs,
		})
	}

	rss := Rss{
		Version: "2.0",
		Newznab: "http://www.newznab.com/DTD/2010/feeds/attributes/",
		Channel: Channel{
			Title:       "example.com",
			Description: "example.com API results",
			NewznabResponse: NewznabResponse{
				Offset: 0,
				Total:  len(Albums),
			},
			Items: items,
		},
	}

	return &rss, nil
}

func fakenzb(w http.ResponseWriter, u url.URL) {
	TidalID := u.Query().Get("tidalid")
	NumTracks := u.Query().Get("numtracks")
	w.Header().Set("Content-Type", "application/x-nzb")
	response := "<?xml version=\"1.0\" encoding=\"UTF-8\" ?>\n" +
		"<!DOCTYPE nzb PUBLIC \"-//newzBin//DTD NZB 1.0//EN\" \"http://www.newzbin.com/DTD/nzb/nzb-1.0.dtd\">\n" +
		"<!-- " + TidalID + "  -->\n" +
		"<!-- " + NumTracks + " -->\n" +
		"<nzb>\n" +
		"    <file post_id=\"1\">\n" +
		"        <groups>\n" +
		"            <group>tidlarr</group>\n" +
		"        </groups>\n" +
		"        <segments>\n" +
		"            <segment number=\"1\">ExampleSegmentID@news.example.com</segment>\n" +
		"        </segments>\n" +
		"    </file>\n" +
		"</nzb>"
	w.Write([]byte(response))
}
