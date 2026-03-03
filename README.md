# youtubedl

Maintained fork of `github.com/steino/youtubedl`, with fixes and adjustments to keep the library working with recent YouTube changes.

The original project is a wrapper around the YouTube Innertube API, heavily inspired by `github.com/kkdai/youtube/v2`.

## Why this fork exists

- keep the project usable while the upstream is not being maintained
- fix recent breaks in signature deciphering
- simplify dependencies and remove legacy pieces
- preserve compatibility with the existing public API

## Requirements

- Go 1.25+

## Installation

Install with:

```bash
go get github.com/ruizlenato/youtubedl@latest
```

## Quick usage

```go
package main

import (
	"io"
	"os"

	"github.com/ruizlenato/youtubedl"
)

func main() {
	client, err := youtubedl.New()
	if err != nil {
		panic(err)
	}

	video, err := client.GetVideo("https://www.youtube.com/watch?v=OjNpRbNdR7E")
	if err != nil {
		panic(err)
	}

	formats := video.Formats.WithAudioChannels().Type("video/mp4")
	formats.Sort()
	if len(formats) == 0 {
		panic("no format found")
	}

	stream, _, err := client.GetStream(video, &formats[0])
	if err != nil {
		panic(err)
	}
	defer stream.Close()

	f, err := os.Create("video.mp4")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if _, err := io.Copy(f, stream); err != nil {
		panic(err)
	}
}
```

## Main API

- `New(opts ...ClientOpts) (*Client, error)`
- `WithHTTPClient(*http.Client) ClientOpts`
- `(*Client).LoadCookies(path string) error`
- `(*Client).GetVideo(urlOrID string, opts ...VideoOpts) (*Video, error)`
- `(*Client).GetStream(video *Video, format *Format) (io.ReadCloser, int64, error)`
- `(*Client).GetStreamURL(video *Video, format *Format) (string, error)`
- `(*Client).GetPlaylist(url string, opts ...VideoOpts) (*Playlist, error)`
- `WithClient(client string) VideoOpts`
