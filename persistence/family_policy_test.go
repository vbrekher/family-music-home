package persistence

import (
	"context"
	"encoding/json"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/id"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pocketbase/dbx"
)

var _ = Describe("Protected family policy persistence", func() {
	It("atomically uses normal password encryption, exact grants and a backend-only policy", func() {
		ctx := context.Background()
		parentID, childID := id.NewRandom(), id.NewRandom()
		sql := GetDBXBuilder()
		ds := &SQLStore{db: sql}
		DeferCleanup(func() {
			for _, q := range []string{`DELETE FROM family_child_profile WHERE user_id={:child}`, `DELETE FROM annotation WHERE user_id={:child}`, `DELETE FROM user_library WHERE user_id IN ({:child},{:parent})`, `DELETE FROM user WHERE id IN ({:child},{:parent})`} {
				_, e := sql.NewQuery(q).Bind(dbx.Params{"child": childID, "parent": parentID}).Execute()
				Expect(e).ToNot(HaveOccurred())
			}
		})
		parent := model.User{ID: parentID, UserName: "parent-unit-" + parentID, Name: "Synthetic parent", IsAdmin: true, NewPassword: "synthetic-parent-password"}
		Expect(ds.User(ctx).Put(&parent)).To(Succeed())
		plain := "synthetic-child-password"
		child := model.User{ID: childID, UserName: "child-unit-" + childID, Name: "Synthetic child", NewPassword: plain}
		body := `{"interface":"kids","density":"large","storyLibraries":[],"allowFamilyMessages":false,"enabled":true}`
		Expect(WithFamilyTransaction(ds, func(tx model.DataStore, b dbx.Builder) error {
			if e := tx.User(ctx).Put(&child); e != nil {
				return e
			}
			if e := tx.User(ctx).SetUserLibraries(childID, []int{}); e != nil {
				return e
			}
			_, e := b.NewQuery(`INSERT INTO family_child_profile(user_id,parent_id,body,created_at,updated_at) VALUES({:child},{:parent},{:body},'2026-10-31','2026-10-31')`).Bind(dbx.Params{"child": childID, "parent": parentID, "body": body}).Execute()
			return e
		})).To(Succeed())
		actual, e := ds.User(ctx).FindByUsernameWithPassword(child.UserName)
		Expect(e).ToNot(HaveOccurred())
		Expect(actual.Password).To(Equal(plain))
		Expect(actual.FamilyChild).ToNot(BeNil())
		Expect(actual.FamilyChild.ParentID).To(Equal(parentID))
		Expect(actual.Libraries).To(BeEmpty())
		var row struct{ Password string }
		Expect(sql.NewQuery(`SELECT password FROM user WHERE id={:id}`).Bind(dbx.Params{"id": childID}).One(&row)).To(Succeed())
		Expect(row.Password).ToNot(Equal(plain))
		public, e := json.Marshal(actual)
		Expect(e).ToNot(HaveOccurred())
		Expect(string(public)).ToNot(ContainSubstring("parentId"))
		Expect(string(public)).ToNot(ContainSubstring("synthetic-child-password"))
		actual.IsAdmin = true
		Expect(ds.User(ctx).Put(actual)).ToNot(Succeed())
		stored, e := ds.User(ctx).Get(childID)
		Expect(e).ToNot(HaveOccurred())
		Expect(stored.IsAdmin).To(BeFalse())
		parent.NewPassword = ""
		parent.IsAdmin = false
		Expect(ds.User(ctx).Put(&parent)).ToNot(Succeed())
		_, e = sql.NewQuery(`UPDATE family_child_profile SET body=json_set(body,'$.interface','full') WHERE user_id={:id}`).Bind(dbx.Params{"id": childID}).Execute()
		Expect(e).ToNot(HaveOccurred())
		stored, e = ds.User(ctx).Get(childID)
		Expect(e).ToNot(HaveOccurred())
		Expect(stored.ID).To(Equal(childID))
		Expect(stored.FamilyChild.Interface).To(Equal("full"))
		Expect(stored.IsAdmin).To(BeFalse())
	})
})
