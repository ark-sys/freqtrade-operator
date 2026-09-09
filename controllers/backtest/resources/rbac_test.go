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

// Regression test: this Role previously granted only configmaps verbs, missing pods:get entirely
// - collectresults.waitForMainContainerExit needs to Get its own Pod to learn when the main
// freqtrade container has exited (see BuildSidecarRole's own doc comment for the full story).
// Found directly: a real Backtest's sidecar crash-looped forever on "cannot get resource pods"
// before this rule existed - nothing in this package's own unit tests (which only ever inspect
// the Role object, never make a real API call under the sidecar's actual identity) could have
// caught this on its own.
func TestBuildSidecarRole_GrantsConfigMapAndPodVerbs(t *testing.T) {
	role := BuildSidecarRole(testNamespace)
	if len(role.Rules) != 2 {
		t.Fatalf("expected exactly two rules, got %d: %+v", len(role.Rules), role.Rules)
	}

	granted := map[string]map[string]bool{}
	for _, rule := range role.Rules {
		for _, resource := range rule.Resources {
			if granted[resource] == nil {
				granted[resource] = map[string]bool{}
			}
			for _, verb := range rule.Verbs {
				granted[resource][verb] = true
			}
		}
	}

	wantConfigMapVerbs := []string{"get", "create", "update"}
	for _, v := range wantConfigMapVerbs {
		if !granted["configmaps"][v] {
			t.Errorf("expected configmaps:%s to be granted, got %+v", v, granted["configmaps"])
		}
	}
	if !granted["pods"]["get"] {
		t.Errorf("expected pods:get to be granted, got %+v", granted["pods"])
	}
	if granted["pods"]["create"] || granted["pods"]["update"] || granted["pods"]["delete"] || granted["pods"]["list"] {
		t.Errorf("expected pods access to be read-only (get only), got %+v", granted["pods"])
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
