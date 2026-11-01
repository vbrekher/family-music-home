package family

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestScheduledLetterPrivacyIdempotenceAndReadReceipt(t *testing.T) {
	s, owner, member, _ := fixture(t)
	now := time.Now().UTC()
	x, e := s.Save(owner, Entry{Kind: "letter", Title: "A private music letter", SharedWith: []string{"member"}, Body: Content{State: "draft", Message: `<p onclick="steal()">Hello <strong>family</strong><img src="https://tracker.invalid"><script>evil()</script></p>`, Timezone: "Europe/Kyiv", DeliveryAt: now.Add(time.Hour).Format(time.RFC3339)}})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(x.Body.Message, "onclick") || strings.Contains(x.Body.Message, "<img") || strings.Contains(x.Body.Message, "<script") {
		t.Fatal("active rich content retained", x.Body.Message)
	}
	if _, e = s.Get(member, x.ID); !errors.Is(e, ErrDenied) {
		t.Fatal("draft letter leaked", e)
	}
	x, e = s.SendLetter(owner, x.ID, x.Revision, "test_unique_send_key_0001", false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(member, x.ID); !errors.Is(e, ErrDenied) {
		t.Fatal("future letter leaked", e)
	}
	if e = s.DeliverDue(context.Background(), now.Add(2*time.Hour)); e != nil {
		t.Fatal(e)
	}
	inbox, e := s.Get(member, x.ID)
	if e != nil || !inbox.Unread || inbox.Body.Message == "" {
		t.Fatal("recipient cannot see delivered letter", inbox, e)
	}
	if e = s.MarkLetterRead(member, x.ID); e != nil {
		t.Fatal(e)
	}
	inbox, e = s.Get(member, x.ID)
	if e != nil || inbox.Unread {
		t.Fatal("read receipt missing", inbox, e)
	}
	if e = s.DeliverDue(context.Background(), now.Add(3*time.Hour)); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SendLetter(owner, x.ID, x.Revision, "test_unique_send_key_0001", false); e != nil {
		t.Fatal(e)
	}
	var n int
	if e = s.DB.QueryRow(`SELECT count(*) FROM family_delivery WHERE entry_id=?`, x.ID).Scan(&n); e != nil || n != 1 {
		t.Fatal("duplicate delivery", n, e)
	}
	if _, e = s.Save(owner, x); !errors.Is(e, ErrConflict) {
		t.Fatal("scheduled content edited without cancellation", e)
	}
}
func TestCapsuleDraftDatesAndExplicitPublishing(t *testing.T) {
	s, o, w, _ := fixture(t)
	x, e := s.Save(o, Entry{Kind: "capsule", Title: "Our memory 🐰", SharedWith: []string{"member"}, Body: Content{State: "draft", DateFrom: "2020-02-29", DateTo: "2020-03-01", Timezone: "Europe/Kyiv", References: []Reference{{Kind: "album", ID: "album"}}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(w, x.ID); !errors.Is(e, ErrDenied) {
		t.Fatal("draft memory leaked", e)
	}
	x.Body.State = "published"
	x, e = s.Save(o, x)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(w, x.ID); e != nil {
		t.Fatal("published memory missing", e)
	}
	x.Body.DateTo = "2019-01-01"
	if _, e = s.Save(o, x); !errors.Is(e, ErrInvalid) {
		t.Fatal("invalid range accepted", e)
	}
}
func TestMusicRequestOwnerTransitionsAndIndependentOutcomes(t *testing.T) {
	s, o, w, _ := fixture(t)
	x, e := s.Save(w, Entry{Kind: "request", Title: "Please add these", Body: Content{Items: []RequestItem{{Kind: "track", Query: "One"}, {Kind: "album", Query: "Two"}}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(o, x.ID); !errors.Is(e, ErrDenied) {
		t.Fatal("unsubmitted wishlist exposed", e)
	}
	x, e = s.SubmitRequest(w, x.ID, x.Revision, []string{"owner"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.UpdateRequest(w, x.ID, x.Body.Items[0].ID, "added", "", x.Revision, nil); !errors.Is(e, ErrDenied) {
		t.Fatal("member acquired owner power", e)
	}
	x, e = s.UpdateRequest(o, x.ID, x.Body.Items[0].ID, "reviewing", "Already in our catalog", x.Revision, nil)
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.UpdateRequest(o, x.ID, x.Body.Items[0].ID, "added", "Resolved", x.Revision, &Reference{Kind: "track", ID: "song"})
	if e != nil {
		t.Fatal(e)
	}
	if x.Body.Items[0].State != "added" || x.Body.Items[1].State != "requested" {
		t.Fatal("bulk outcomes not independent", x)
	}
	h, e := s.RequestHistory(w, x.ID)
	if e != nil || len(h) != 4 {
		t.Fatal("history missing", len(h), e)
	}
}
func TestAnniversaryTimezoneAndLeapDay(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	next, e := NextAnniversary(now, "02-29", "09:00", "Europe/Kyiv")
	if e != nil || next.Year() != 2028 || next.Month() != 2 || next.Day() != 29 {
		t.Fatal(next, e)
	}
	if _, e = NextAnniversary(now, "02-30", "09:00", "UTC"); !errors.Is(e, ErrInvalid) {
		t.Fatal("invalid anniversary accepted", e)
	}
}
func TestCancellationAndAnnualResendDoNotDuplicateRecurringSeries(t *testing.T) {
	s, o, w, _ := fixture(t)
	now := time.Now().UTC()
	x, e := s.Save(o, Entry{Kind: "letter", Title: "Annual hello", SharedWith: []string{"member"}, Body: Content{State: "draft", Message: "Hello", Timezone: "UTC", DeliveryAt: now.Add(time.Minute).Format(time.RFC3339), Anniversary: "12-25", AnniversaryTime: "10:00"}})
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.SendLetter(o, x.ID, x.Revision, "annual_first_key_00001", false)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.DeliverDue(context.Background(), now.Add(2*time.Minute)); e != nil {
		t.Fatal(e)
	}
	x, e = s.Get(o, x.ID)
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.SendLetter(o, x.ID, x.Revision, "annual_resend_key_0001", true)
	if e != nil {
		t.Fatal(e)
	}
	var pending int
	if e = s.DB.QueryRow(`SELECT count(*) FROM family_delivery WHERE entry_id=? AND delivered_at IS NULL`, x.ID).Scan(&pending); e != nil || pending != 1 {
		t.Fatal("resend duplicated annual schedule", pending, e)
	}
	x, e = s.CancelLetter(o, x.ID, x.Revision)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.DB.QueryRow(`SELECT count(*) FROM family_delivery WHERE entry_id=? AND delivered_at IS NULL`, x.ID).Scan(&pending); e != nil || pending != 0 {
		t.Fatal("future delivery not cancelled", pending, e)
	}
	if _, e = s.Get(w, x.ID); e != nil {
		t.Fatal("cancel removed delivered letter", e)
	}
}
func TestRequestCancellationPreservesResolutionHiddenFromRequester(t *testing.T) {
	s, o, w, allowed := fixture(t)
	x, e := s.Save(w, Entry{Kind: "request", Title: "Two requests", Body: Content{Items: []RequestItem{{Kind: "track", Query: "One"}, {Kind: "track", Query: "Two"}}}})
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.SubmitRequest(w, x.ID, x.Revision, []string{"owner"})
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.UpdateRequest(o, x.ID, x.Body.Items[0].ID, "reviewing", "", x.Revision, nil)
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.UpdateRequest(o, x.ID, x.Body.Items[0].ID, "added", "", x.Revision, &Reference{Kind: "track", ID: "song"})
	if e != nil {
		t.Fatal(e)
	}
	*allowed = false
	x, e = s.Get(w, x.ID)
	if e != nil {
		t.Fatal(e)
	}
	if x.Body.Items[0].Resolution != nil {
		t.Fatal("revoked resolution leaked")
	}
	x, e = s.UpdateRequest(w, x.ID, x.Body.Items[1].ID, "cancelled", "No longer needed", x.Revision, nil)
	if e != nil {
		t.Fatal(e)
	}
	var stored string
	if e = s.DB.QueryRow(`SELECT json_extract(body,'$.items[0].resolution.id') FROM family_entry WHERE id=?`, x.ID).Scan(&stored); e != nil || stored != "song" {
		t.Fatal("redacted reader view overwrote canonical resolution", stored, e)
	}
}

func TestReferenceURLsAndNotificationPrivacy(t *testing.T) {
	for _, url := range []string{"https://localhost/music", "https://192.168.1.1/music", "https://name:secret@example.com/", "https://example.com/?X-Amz-Signature=secret", "javascript:alert(1)", "http://example.com/"} {
		if safeReferenceURL(url) {
			t.Fatal("unsafe reference accepted", url)
		}
	}
	if !safeReferenceURL("https://musicbrainz.org/release/test") {
		t.Fatal("ordinary public reference rejected")
	}
	s, o, w, _ := fixture(t)
	x, e := s.Save(o, Entry{Kind: "letter", Title: "Notification privacy", SharedWith: []string{"member"}, Body: Content{State: "draft", Timezone: "UTC", Message: "Hello"}})
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.SendLetter(o, x.ID, x.Revision, "notifyprivacy0001", false)
	if e != nil {
		t.Fatal(e)
	}
	notes, e := s.Notifications(w)
	if e != nil || len(notes) != 1 {
		t.Fatal(notes, e)
	}
	if _, e = s.DB.Exec(`DELETE FROM family_access WHERE entry_id=?`, x.ID); e != nil {
		t.Fatal(e)
	}
	notes, e = s.Notifications(w)
	if e != nil || len(notes) != 0 {
		t.Fatal("revoked letter notification leaked", notes, e)
	}
}
