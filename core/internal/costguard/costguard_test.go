package costguard

import (
	"math"
	"os"
	"strings"
	"testing"
)

const testRules = `
apiVersion: forgelab/v1
kind: CostGuard
spec:
  maxMonthlyUsd: 100
  maxTtlHours: 72
  requiredTags: [forgelab-owner, forgelab-ttl-hours]
  deniedTypes: [aws_nat_gateway]
  freeTypes: [aws_vpc]
  pricing:
    - type: aws_eks_cluster
      fixedHourlyUsd: 0.10
    - type: aws_eks_node_group
      instanceAttribute: instance_types.0
      countAttribute: scaling_config.0.desired_size
      hourlyUsd: {t3.small: 0.02, t3.medium: 0.04}
      spotAttribute: capacity_type
      spotValue: SPOT
      spotFactor: 0.5
    - type: google_redis_instance
      fixedHourlyUsd: 0.05
      perGbAttribute: memory_size_gb
`

func rules(t *testing.T) *Rules {
	t.Helper()
	r, err := ParseRules([]byte(testRules))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.01 }

const goodPlan = `{"resource_changes":[
 {"address":"aws_vpc.this","type":"aws_vpc","change":{"actions":["create"],"after":{"tags_all":{"forgelab-owner":"me","forgelab-ttl-hours":"8"}}}},
 {"address":"aws_eks_cluster.this","type":"aws_eks_cluster","change":{"actions":["create"],"after":{"tags_all":{"forgelab-owner":"me","forgelab-ttl-hours":"8"}}}},
 {"address":"aws_eks_node_group.workers","type":"aws_eks_node_group","change":{"actions":["create"],"after":{
   "instance_types":["t3.medium"],"capacity_type":"ON_DEMAND","scaling_config":[{"desired_size":2}],
   "tags_all":{"forgelab-owner":"me","forgelab-ttl-hours":"8"}}}},
 {"address":"aws_iam_role_policy_attachment.x","type":"aws_iam_role_policy_attachment","change":{"actions":["create"],"after":{"role":"r"}}},
 {"address":"aws_old.thing","type":"aws_old","change":{"actions":["delete"],"after":null}}
]}`

func TestEvaluatePasses(t *testing.T) {
	rep, err := Evaluate(rules(t), []byte(goodPlan))
	if err != nil {
		t.Fatal(err)
	}
	// cluster 0.10*730 = 73 ; node group 0.04*2*730 = 58.4 -> 131.4 exceeds the 100 ceiling.
	if !near(rep.MonthlyUSD, 131.4) {
		t.Fatalf("monthly = %v", rep.MonthlyUSD)
	}
	if rep.OK() || len(rep.Violations) != 1 || rep.Violations[0].Rule != "max-monthly" {
		t.Fatalf("violations = %+v", rep.Violations)
	}
	if len(rep.Warnings) != 1 || rep.Warnings[0].Rule != "unpriced" {
		t.Fatalf("warnings = %+v", rep.Warnings)
	}
}

func TestSpotDiscountAndCeiling(t *testing.T) {
	plan := strings.Replace(goodPlan, `"ON_DEMAND"`, `"SPOT"`, 1)
	r := rules(t)
	r.Spec.MaxMonthlyUSD = 500
	rep, _ := Evaluate(r, []byte(plan))
	if !near(rep.MonthlyUSD, 73+29.2) || !rep.OK() {
		t.Fatalf("monthly = %v violations = %+v", rep.MonthlyUSD, rep.Violations)
	}
}

func TestDeniedType(t *testing.T) {
	plan := `{"resource_changes":[{"address":"aws_nat_gateway.n","type":"aws_nat_gateway","change":{"actions":["create"],"after":{}}}]}`
	rep, _ := Evaluate(rules(t), []byte(plan))
	if rep.OK() || rep.Violations[0].Rule != "denied-type" {
		t.Fatalf("violations = %+v", rep.Violations)
	}
}

func TestRequiredTagsAndTTL(t *testing.T) {
	plan := `{"resource_changes":[
	 {"address":"aws_vpc.a","type":"aws_vpc","change":{"actions":["create"],"after":{"tags_all":{"forgelab-ttl-hours":"8"}}}},
	 {"address":"aws_vpc.b","type":"aws_vpc","change":{"actions":["create"],"after":{"tags_all":{"forgelab-owner":"me","forgelab-ttl-hours":"500"}}}},
	 {"address":"aws_vpc.c","type":"aws_vpc","change":{"actions":["create"],"after":{"tags_all":{"forgelab-owner":"me","forgelab-ttl-hours":"soon"}}}}
	]}`
	rep, _ := Evaluate(rules(t), []byte(plan))
	got := map[string]string{}
	for _, v := range rep.Violations {
		got[v.Address] = v.Rule
	}
	if got["aws_vpc.a"] != "required-tag" || got["aws_vpc.b"] != "ttl" || got["aws_vpc.c"] != "ttl" {
		t.Fatalf("violations = %+v", rep.Violations)
	}
}

func TestUntaggableResourcesSkipTagRule(t *testing.T) {
	plan := `{"resource_changes":[{"address":"aws_iam_role_policy_attachment.x","type":"aws_vpc","change":{"actions":["create"],"after":{"role":"r"}}}]}`
	rep, _ := Evaluate(rules(t), []byte(plan))
	if !rep.OK() {
		t.Fatalf("violations = %+v", rep.Violations)
	}
}

func TestGCPLabelsAndPerGB(t *testing.T) {
	plan := `{"resource_changes":[{"address":"google_redis_instance.c","type":"google_redis_instance","change":{"actions":["create"],"after":{"memory_size_gb":2,
	 "effective_labels":{"forgelab-owner":"me","forgelab-ttl-hours":"8"}}}}]}`
	rep, _ := Evaluate(rules(t), []byte(plan))
	if !near(rep.MonthlyUSD, 0.05*2*730) || !rep.OK() {
		t.Fatalf("monthly = %v violations = %+v", rep.MonthlyUSD, rep.Violations)
	}
}

func TestUnknownInstanceIsViolation(t *testing.T) {
	plan := strings.Replace(goodPlan, "t3.medium", "p4d.24xlarge", 1)
	rep, _ := Evaluate(rules(t), []byte(plan))
	found := false
	for _, v := range rep.Violations {
		if v.Rule == "pricing" && strings.Contains(v.Message, "p4d.24xlarge") {
			found = true
		}
	}
	if !found {
		t.Fatalf("violations = %+v", rep.Violations)
	}
}

func TestFailOnUnpriced(t *testing.T) {
	r := rules(t)
	r.Spec.FailOnUnpriced = true
	plan := `{"resource_changes":[{"address":"aws_thing.x","type":"aws_thing","change":{"actions":["create"],"after":{}}}]}`
	rep, _ := Evaluate(r, []byte(plan))
	if rep.OK() || rep.Violations[0].Rule != "unpriced" {
		t.Fatalf("violations = %+v", rep.Violations)
	}
}

func TestParseRulesErrors(t *testing.T) {
	head := "apiVersion: forgelab/v1\nkind: CostGuard\nspec:\n"
	cases := map[string]string{
		"version":     strings.Replace(testRules, "forgelab/v1", "v0", 1),
		"kind":        strings.Replace(testRules, "CostGuard", "Other", 1),
		"no ceiling":  head + "  maxMonthlyUsd: 0\n",
		"unknown":     head + "  maxMonthlyUsd: 1\n  bogus: 1\n",
		"dup pricing": head + "  maxMonthlyUsd: 1\n  pricing:\n    - {type: a, fixedHourlyUsd: 1}\n    - {type: a, fixedHourlyUsd: 1}\n",
		"instance":    head + "  maxMonthlyUsd: 1\n  pricing:\n    - {type: a, instanceAttribute: x}\n",
		"spot factor": head + "  maxMonthlyUsd: 1\n  pricing:\n    - {type: a, fixedHourlyUsd: 1, spotFactor: 2}\n",
	}
	for name, doc := range cases {
		if _, err := ParseRules([]byte(doc)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := Evaluate(rules(t), []byte("not json")); err == nil {
		t.Error("expected error for invalid plan JSON")
	}
}

// The repository's own rules must accept the presets' default plans; this
// keeps the price table and the presets from drifting apart.
func TestRepositoryRulesAcceptDefaultPresets(t *testing.T) {
	r, err := LoadRules("../../../environments/cloud/cost-guard.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"aws-default-plan.json", "gcp-default-plan.json"} {
		raw, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		rep, err := Evaluate(r, raw)
		if err != nil {
			t.Fatal(err)
		}
		if !rep.OK() {
			t.Errorf("%s: violations = %+v", name, rep.Violations)
		}
		if rep.MonthlyUSD <= 0 || rep.MonthlyUSD > r.Spec.MaxMonthlyUSD {
			t.Errorf("%s: estimate $%.2f outside (0, %.0f]", name, rep.MonthlyUSD, r.Spec.MaxMonthlyUSD)
		}
		t.Logf("%s: $%.2f/month", name, rep.MonthlyUSD)
	}
}

func TestRepositoryRulesRejectOversizedPlan(t *testing.T) {
	r, err := LoadRules("../../../environments/cloud/cost-guard.yaml")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile("testdata/aws-default-plan.json")
	big := strings.Replace(string(raw), `"desired_size": 2`, `"desired_size": 40`, 1)
	big = strings.Replace(big, `"SPOT"`, `"ON_DEMAND"`, 1)
	rep, _ := Evaluate(r, []byte(big))
	if rep.OK() {
		t.Fatalf("expected the ceiling to reject a 40-node on-demand cluster, estimate $%.2f", rep.MonthlyUSD)
	}
}
