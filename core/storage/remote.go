package storage

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/navidrome/navidrome/model"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"
)

// OpenRemote resolves an administrator-configured remote URI without exposing credentials.
func OpenRemote(uri string) (fs.File, error) {
	u, err := url.Parse(uri)
	if err != nil || !model.IsRemotePath(uri) || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fs.ErrInvalid
	}
	name := path.Base(u.Path)
	if name == "." || name == "/" {
		return nil, fs.ErrInvalid
	}
	u.Path = path.Dir(u.Path)
	u.RawPath = ""
	s, err := For(u.String())
	if err != nil {
		return nil, err
	}
	f, err := s.FS()
	if err != nil {
		return nil, err
	}
	return f.Open(name)
}

var bridge struct {
	once sync.Once
	key  [32]byte
	base string
	err  error
}

func bridgeMAC(uri string) string {
	m := hmac.New(sha256.New, bridge.key[:])
	_, _ = m.Write([]byte(uri))
	return hex.EncodeToString(m.Sum(nil))
}

// FFmpegPath returns a loopback-only, process-authenticated Range endpoint for
// a remote object. No Storage credential enters an ffmpeg command or log.
func FFmpegPath(uri string) (string, error) {
	if !model.IsRemotePath(uri) {
		return uri, nil
	}
	bridge.once.Do(func() {
		if _, bridge.err = rand.Read(bridge.key[:]); bridge.err != nil {
			return
		}
		var listener net.Listener
		listener, bridge.err = net.Listen("tcp", "127.0.0.1:0")
		if bridge.err != nil {
			return
		}
		bridge.base = "http://" + listener.Addr().String() + "/"
		srv := &http.Server{Handler: http.HandlerFunc(serveRemote), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: time.Minute}
		go func() { _ = srv.Serve(listener) }()
	})
	if bridge.err != nil {
		return "", errors.New("remote audio bridge unavailable")
	}
	return bridge.base + "?" + url.Values{"uri": {uri}, "token": {bridgeMAC(uri)}}.Encode(), nil
}
func serveRemote(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}
	uri := r.URL.Query().Get("uri")
	expected, err := hex.DecodeString(bridgeMAC(uri))
	if err != nil {
		w.WriteHeader(403)
		return
	}
	actual, err := hex.DecodeString(r.URL.Query().Get("token"))
	if err != nil || !hmac.Equal(actual, expected) {
		w.WriteHeader(403)
		return
	}
	f, err := OpenRemote(uri)
	if err != nil {
		http.Error(w, "Music storage unavailable", 503)
		return
	}
	defer f.Close()
	seeker, ok := f.(io.ReadSeeker)
	if !ok {
		w.WriteHeader(503)
		return
	}
	info, err := f.Stat()
	if err != nil {
		w.WriteHeader(503)
		return
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), seeker)
}
