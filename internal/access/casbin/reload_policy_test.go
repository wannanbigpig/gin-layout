package casbinx

import (
	"errors"
	"testing"

	"github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
)

type failingPolicyAdapter struct{}

func (failingPolicyAdapter) LoadPolicy(casbinmodel.Model) error          { return errors.New("load failed") }
func (failingPolicyAdapter) SavePolicy(casbinmodel.Model) error          { return nil }
func (failingPolicyAdapter) AddPolicy(string, string, []string) error    { return nil }
func (failingPolicyAdapter) RemovePolicy(string, string, []string) error { return nil }
func (failingPolicyAdapter) RemoveFilteredPolicy(string, string, int, ...string) error {
	return nil
}

func TestReloadPolicyFailureMakesEnforcementFailClosed(t *testing.T) {
	m, err := casbinmodel.NewModelFromString(`
[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
`)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatalf("build enforcer: %v", err)
	}
	if ok, addErr := enforcer.AddPolicy("admin::2", "/v1/users", "GET"); addErr != nil || !ok {
		t.Fatalf("add policy: ok=%v err=%v", ok, addErr)
	}
	enforcer.SetAdapter(failingPolicyAdapter{})

	managed := &CasbinEnforcer{
		Enforcer:    enforcer,
		policyState: &sharedPolicyState{},
	}
	if err := managed.reloadPolicy(); err == nil {
		t.Fatal("expected reload failure")
	}
	if ok, enforceErr := managed.Enforce("admin::2", "/v1/users", "GET"); enforceErr == nil || ok {
		t.Fatalf("expected fail-closed enforcement, ok=%v err=%v", ok, enforceErr)
	}
}
