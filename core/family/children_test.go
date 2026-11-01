package family

import (
	"context"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"testing"
)

func TestChildSharingNeedsExplicitParentApprovalEvenWithStaleGrant(t *testing.T) {
	s, owner, member, _ := fixture(t)
	p := &model.FamilyChild{ParentID: "owner", Enabled: true, Interface: "kids", Density: "large"}
	child := request.WithUser(context.Background(), model.User{ID: "child", FamilyChild: p})
	a, e := s.Save(owner, Entry{Kind: "shelf", Title: "Parent approved", SharedWith: []string{"child"}, Body: Content{References: []Reference{}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(child, a.ID); e != nil {
		t.Fatal(e)
	}
	b, e := s.Save(member, Entry{Kind: "shelf", Title: "Other adult", SharedWith: []string{"child"}, Body: Content{References: []Reference{}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(child, b.ID); e != ErrDenied {
		t.Fatal("stale grant widened child policy", e)
	}
	if e = s.CanRead(child, b.ID); e != ErrDenied {
		t.Fatal("attachment parent gate", e)
	}
	list, e := s.List(child, "shelf", false, 0)
	if e != nil || len(list) != 1 || list[0].ID != a.ID {
		t.Fatal(list, e)
	}
	p.AllowFamilyMessages = true
	if _, e = s.Get(child, b.ID); e != nil {
		t.Fatal(e)
	}
	p.Enabled = false
	if _, e = s.Get(child, a.ID); e != ErrDenied {
		t.Fatal("paused child read", e)
	}
}
func TestChildPolicyKeepsStoryLibrariesInsideCanonicalAllowlist(t *testing.T) {
	p := model.FamilyChild{Interface: "kids", Density: "large", Enabled: true, StoryLibraries: []int{2}}
	if ValidateChildPolicy(p, []int{1}) == nil {
		t.Fatal("story bypass")
	}
	if e := ValidateChildPolicy(p, []int{1, 2}); e != nil {
		t.Fatal(e)
	}
	p.Interface = "full"
	if e := ValidateChildPolicy(p, []int{1, 2}); e != nil {
		t.Fatal(e)
	}
	t.Setenv("BUNNY_MEMBER_USERNAME", "reserved-member")
	if ValidateChildName("reserved-member", "Example") == nil {
		t.Fatal("existing family account conversion")
	}
}
