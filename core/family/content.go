package family

import (
	"context"
	"encoding/json"
	"html"
	"net/netip"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/navidrome/navidrome/model/id"
	xhtml "golang.org/x/net/html"
)

type RequestItem struct {
	Candidates []Reference `json:"candidates,omitempty"`
	ID         string      `json:"id"`
	Kind       string      `json:"kind"`
	Query      string      `json:"query"`
	URL        string      `json:"url,omitempty"`
	State      string      `json:"state"`
	Resolution *Reference  `json:"resolution,omitempty"`
}
type Delivery struct {
	RecipientID string `json:"recipientId"`
	DueAt       int64  `json:"dueAt"`
	DeliveredAt *int64 `json:"deliveredAt"`
	ReadAt      *int64 `json:"readAt"`
}

// Rich text admits formatting only: no links, images, styles, event handlers or
// external resource attributes. The WebUI sanitizes again before rendering.
func SanitizeMessage(input string) string {
	allowed := map[string]bool{"p": true, "br": true, "strong": true, "b": true, "em": true, "i": true, "ul": true, "ol": true, "li": true, "blockquote": true}
	z := xhtml.NewTokenizer(strings.NewReader(input))
	var out strings.Builder
	for {
		tt := z.Next()
		if tt == xhtml.ErrorToken {
			break
		}
		switch tt {
		case xhtml.TextToken:
			out.WriteString(html.EscapeString(string(z.Text())))
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			tag, _ := z.TagName()
			if allowed[string(tag)] {
				out.WriteString("<" + string(tag) + ">")
			}
		case xhtml.EndTagToken:
			tag, _ := z.TagName()
			if allowed[string(tag)] && string(tag) != "br" {
				out.WriteString("</" + string(tag) + ">")
			}
		}
	}
	return out.String()
}
func validKind(kind string) bool {
	return kind == "shelf" || kind == "capsule" || kind == "letter" || kind == "request"
}
func validateDateRange(from, to, zone string) error {
	if _, e := time.LoadLocation(zone); e != nil {
		return ErrInvalid
	}
	a, e := time.Parse("2006-01-02", from)
	if e != nil {
		return ErrInvalid
	}
	if to == "" {
		return nil
	}
	b, e := time.Parse("2006-01-02", to)
	if e != nil || b.Before(a) {
		return ErrInvalid
	}
	return nil
}
func safeReferenceURL(raw string) bool {
	if raw == "" {
		return true
	}
	if len(raw) > 2048 || strings.ContainsAny(raw, "\r\n\x00") {
		return false
	}
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") {
		return false
	}
	if ip, e := netip.ParseAddr(host); e == nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()) {
		return false
	}
	for key := range u.Query() {
		switch strings.ToLower(key) {
		case "password", "access_token", "api_key", "x-amz-signature", "x-amz-security-token", "signature":
			return false
		}
	}
	return true
}

func (s Store) prepareContent(ctx context.Context, x *Entry, old *Entry) error {
	b := &x.Body
	switch x.Kind {
	case "shelf":
		*b = Content{Description: b.Description, References: b.References, Cover: b.Cover, Hidden: b.Hidden}
	case "capsule":
		if b.State != "draft" && b.State != "published" {
			return ErrInvalid
		}
		if e := validateDateRange(b.DateFrom, b.DateTo, b.Timezone); e != nil {
			return e
		}
		*b = Content{Description: b.Description, References: b.References, Cover: b.Cover, CoverAttachmentID: b.CoverAttachmentID, State: b.State, DateFrom: b.DateFrom, DateTo: b.DateTo, Timezone: b.Timezone}
	case "letter":
		if old != nil && old.Body.State != "draft" {
			return ErrConflict
		}
		if b.State != "" && b.State != "draft" {
			return ErrInvalid
		}
		if len(b.Message) > 16000 {
			return ErrInvalid
		}
		if _, e := time.LoadLocation(b.Timezone); e != nil {
			return ErrInvalid
		}
		if b.DeliveryAt != "" {
			if _, e := time.Parse(time.RFC3339, b.DeliveryAt); e != nil {
				return ErrInvalid
			}
		}
		if b.Anniversary != "" {
			if _, e := time.Parse("01-02", b.Anniversary); e != nil {
				return ErrInvalid
			}
			if _, e := time.Parse("15:04", b.AnniversaryTime); e != nil {
				return ErrInvalid
			}
		}
		*b = Content{CoverAttachmentID: b.CoverAttachmentID, Message: SanitizeMessage(b.Message), References: b.References, Cover: b.Cover, State: "draft", Timezone: b.Timezone, DeliveryAt: b.DeliveryAt, Anniversary: b.Anniversary, AnniversaryTime: b.AnniversaryTime}
	case "request":
		if old != nil && old.Body.State != "draft" {
			return ErrConflict
		}
		if b.State != "" && b.State != "draft" {
			return ErrInvalid
		}
		if len(b.Items) < 1 || len(b.Items) > 100 {
			return ErrInvalid
		}
		seen := map[string]bool{}
		for i := range b.Items {
			item := &b.Items[i]
			item.Query = strings.TrimSpace(item.Query)
			if len(item.Query) < 1 || len(item.Query) > 300 || item.Kind != "track" && item.Kind != "album" && item.Kind != "artist" || !safeReferenceURL(item.URL) {
				return ErrInvalid
			}
			key := item.Kind + ":" + strings.ToLower(strings.Join(strings.Fields(item.Query), " "))
			if seen[key] {
				return ErrInvalid
			}
			seen[key] = true
			requestedID := item.ID
			item.ID = id.NewRandom()
			if old != nil {
				for _, prev := range old.Body.Items {
					if prev.ID == requestedID {
						item.ID = prev.ID
					}
				}
			}
			item.State = "requested"
			item.Resolution = nil
			item.Candidates = nil
		}
		*b = Content{Description: b.Description, Items: b.Items, State: "draft", References: []Reference{}}
	}
	return nil
}
func decodeStored(body string) (Content, error) {
	var c Content
	e := json.Unmarshal([]byte(body), &c)
	return c, e
}
