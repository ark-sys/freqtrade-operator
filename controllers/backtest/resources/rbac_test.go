package resources

import "testing"

func TestBuildSidecarServiceAccount_FixedNamePerNamespace(t *testing.T) {
	sa := BuildSidecarServiceAccount(testNamespace)
	if sa.Name != SidecarServiceAccountName {
		t.Errorf("expected name %q, got %q", SidecarServiceAccountName, sa.Name)
	}
	if sa.Namespace != testNamespace {
		t.Errorf("expected namespace trading, got %s", sa.Namespace)
	}
}

func TestBuildSidecarRole_GrantsOnlyConfigMapVerbs(t *testing.T) {
	role := BuildSidecarRole(testNamespace)
	if len(role.Rules) != 1 {
		t.Fatalf("expected exactly one rule, got %d: %+v", len(role.Rules), role.Rules)
	}
	rule := role.Rules[0]
	if len(rule.Resources) != 1 || rule.Resources[0] != "configmaps" {
		t.Errorf("expected the rule to be scoped to configmaps only, got %v", rule.Resources)
	}
	wantVerbs := map[string]bool{"get": true, "create": true, "update": true}
	if len(rule.Verbs) != len(wantVerbs) {
		t.Errorf("expected exactly %v, got %v", wantVerbs, rule.Verbs)
	}
	for _, v := range rule.Verbs {
		if !wantVerbs[v] {
			t.Errorf("unexpected verb %q granted", v)
		}
	}
}

func TestBuildSidecarRoleBinding_BindsRoleToServiceAccountInNamespace(t *testing.T) {
	rb := BuildSidecarRoleBinding(testNamespace)
	if rb.RoleRef.Name != SidecarServiceAccountName || rb.RoleRef.Kind != "Role" {
		t.Errorf("expected RoleRef to point at the Role %q, got %+v", SidecarServiceAccountName, rb.RoleRef)
	}
	if len(rb.Subjects) != 1 {
		t.Fatalf("expected exactly one subject, got %+v", rb.Subjects)
	}
	if rb.Subjects[0].Name != SidecarServiceAccountName || rb.Subjects[0].Namespace != testNamespace {
		t.Errorf("expected the subject to name the ServiceAccount in %s, got %+v", testNamespace, rb.Subjects[0])
	}
}

func TestResultsConfigMapName(t *testing.T) {
	if got := ResultsConfigMapName("my-run"); got != "my-run-results" {
		t.Errorf("expected my-run-results, got %q", got)
	}
}
