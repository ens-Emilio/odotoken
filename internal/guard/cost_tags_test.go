package guard

import "testing"

func TestSetAndGetCostTags(t *testing.T) {
	costTags = make(map[string][]CostTag) // reset
	tags := []CostTag{{Key: "team", Value: "backend"}, {Key: "env", Value: "prod"}}
	SetCostTags("client-a", tags)
	got := GetCostTags("client-a")
	if len(got) != 2 || got[0].Key != "team" {
		t.Errorf("GetCostTags(client-a) = %+v, want 2 tags starting with team", got)
	}
	// unknown entity returns empty slice
	empty := GetCostTags("unknown")
	if len(empty) != 0 {
		t.Errorf("GetCostTags(unknown) = %+v, want empty", empty)
	}
}

func TestMergeTags(t *testing.T) {
	base := []CostTag{{Key: "env", Value: "dev"}, {Key: "team", Value: "infra"}}
	overrides := []CostTag{{Key: "env", Value: "prod"}, {Key: "region", Value: "us-east"}}
	merged := MergeTags(base, overrides)
	m := make(map[string]string, len(merged))
	for _, tag := range merged {
		m[tag.Key] = tag.Value
	}
	if m["env"] != "prod" {
		t.Errorf("env = %q, want prod", m["env"])
	}
	if m["team"] != "infra" {
		t.Errorf("team = %q, want infra", m["team"])
	}
	if m["region"] != "us-east" {
		t.Errorf("region = %q, want us-east", m["region"])
	}
}