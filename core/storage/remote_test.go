package storage

import (
	"github.com/navidrome/navidrome/model/metadata"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
)

type bridgeTestFS struct{ fs.FS }

func (f bridgeTestFS) ReadTags(...string) (map[string]metadata.Info, error) { return nil, nil }

type bridgeTestStorage struct{ MusicFS }

func (s bridgeTestStorage) FS() (MusicFS, error) { return s.MusicFS, nil }
func TestRemoteBridgeRangeAndAuthorization(t *testing.T) {
	data := strings.Repeat("lossless-data", 1000)
	Register("bunny", func(u url.URL) Storage {
		return bridgeTestStorage{bridgeTestFS{fstest.MapFS{"track.flac": {Data: []byte(data)}}}}
	})
	endpoint, err := FFmpegPath("bunny://test/music/track.flac")
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Range", "bytes=123-456")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != 206 || string(b) != data[123:457] {
		t.Fatalf("range response %d, len %d", r.StatusCode, len(b))
	}
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("token", "00")
	u.RawQuery = q.Encode()
	r, err = http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 403 {
		t.Fatalf("forged token accepted: %d", r.StatusCode)
	}
	q = u.Query()
	q.Set("uri", "bunny://test/music/private.flac")
	u.RawQuery = q.Encode()
	r, err = http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 403 {
		t.Fatal("token reused for another object")
	}
	if _, err = OpenRemote("https://example.org/music"); err == nil {
		t.Fatal("arbitrary remote scheme accepted")
	}
}

func TestRemoteOpenLiteralEscapedNames(t *testing.T) {
	for _, name := range []string{"100% song.mp3", "a%23b.mp3", "a#b.mp3", "a%2Fb.mp3"} {
		Register("bunny", func(u url.URL) Storage {
			if u.Path != "/music/album#%" {
				t.Errorf("wrong parent path %q", u.Path)
			}
			return bridgeTestStorage{bridgeTestFS{fstest.MapFS{name: {Data: []byte(name)}}}}
		})
		u := url.URL{Scheme: "bunny", Host: "test", Path: "/music/album#%/" + name}
		f, e := OpenRemote(u.String())
		if e != nil {
			t.Fatal(e)
		}
		b, e := io.ReadAll(f)
		f.Close()
		if e != nil || string(b) != name {
			t.Fatalf("wrong object %q %v", b, e)
		}
	}
}
