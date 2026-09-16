package admin

import (
	"errors"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"testing"
)

func TestAdminUserMutationSuperAdminBoundary(t *testing.T) {
	for _, tc := range []struct {
		name                               string
		actorSuper, targetSuper, requested *uint8
		targetID                           uint
		wantDenied                         bool
	}{
		{name: "ordinary create explicit zero", requested: bytePtr(0)},
		{name: "ordinary create omitted"},
		{name: "ordinary cannot grant super", requested: bytePtr(1), wantDenied: true},
		{name: "ordinary cannot reset extra super password", targetID: 20, targetSuper: bytePtr(1), wantDenied: true},
		{name: "ordinary cannot reset extra super with unchanged flag", targetID: 20, targetSuper: bytePtr(1), requested: bytePtr(1), wantDenied: true},
		{name: "super may reset extra super", actorSuper: bytePtr(1), targetID: 20, targetSuper: bytePtr(1)},
		{name: "ordinary may reset ordinary", targetID: 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewAdminUserService()
			s.SetAdminUserId(10)
			actor := &model.AdminUser{}
			if tc.actorSuper != nil {
				actor.IsSuperAdmin = *tc.actorSuper
			}
			defer replaceSuperAdminCheckLoader(t, actor, nil)()
			target := &model.AdminUser{ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{BaseModel: model.BaseModel{ID: tc.targetID}}}
			if tc.targetSuper != nil {
				target.IsSuperAdmin = *tc.targetSuper
			}
			err := s.authorizeAdminUserMutation(target, &adminUserEditParams{Id: tc.targetID, Password: stringPtr("new-password"), IsSuperAdmin: tc.requested})
			if (err != nil) != tc.wantDenied {
				t.Fatalf("denied=%v, want %v: %v", err != nil, tc.wantDenied, err)
			}
		})
	}
}
func bytePtr(v uint8) *uint8 { return &v }

func TestOrdinaryAdminCreateExplicitZeroPassesAuthorization(t *testing.T) {
	s := NewAdminUserService()
	s.SetAdminUserId(10)
	defer replaceSuperAdminCheckLoader(t, &model.AdminUser{IsSuperAdmin: global.No}, nil)()
	zero := uint8(0)
	// Stop at required-field validation, before any database operation.
	err := s.saveAdminUserMutation(&adminUserEditParams{IsSuperAdmin: &zero})
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) || businessErr.GetCode() != e.UsernameRequired {
		t.Fatalf("expected required username validation, got %v", err)
	}
}
