package fingerprint

import "testing"

func TestParseAndMatchTypeRule(t *testing.T) {
	rules, err := ParseRules(`
# group db outages
error.type:DatabaseUnavailable -> system-down
error.value:"connection error: *" -> connection-error, {{ transaction }}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("rules=%d", len(rules))
	}
	hit := Match(rules, Input{ExceptionType: "DatabaseUnavailable", Message: "x"})
	if hit == nil || hit.Fingerprint[0] != "system-down" {
		t.Fatalf("hit=%+v", hit)
	}
	hit = Match(rules, Input{ExceptionType: "Other", Message: "connection error: host=1", Transaction: "/api"})
	if hit == nil || hit.Fingerprint[0] != "connection-error" {
		t.Fatalf("value rule hit=%+v", hit)
	}
}

func TestFirstMatchWins(t *testing.T) {
	rules, err := ParseRules("error.type:Foo -> first\nerror.type:Foo -> second")
	if err != nil {
		t.Fatal(err)
	}
	hit := Match(rules, Input{ExceptionType: "Foo"})
	if hit == nil || hit.Fingerprint[0] != "first" {
		t.Fatalf("hit=%+v", hit)
	}
}

func TestFrameConjunction(t *testing.T) {
	rules, err := ParseRules(`error.type:ConnectionError stack.function:connect stack.module:bot -> bot-error`)
	if err != nil {
		t.Fatal(err)
	}
	miss := Match(rules, Input{
		ExceptionType: "ConnectionError",
		Frames: []Frame{
			{Function: "connect", Module: "other", InApp: true},
			{Function: "other", Module: "bot", InApp: true},
		},
	})
	if miss != nil {
		t.Fatal("frame matchers must hit the same frame")
	}
	hit := Match(rules, Input{
		ExceptionType: "ConnectionError",
		Frames: []Frame{
			{Function: "connect", Module: "bot", InApp: true},
		},
	})
	if hit == nil {
		t.Fatal("expected match on same frame")
	}
}

func TestNegateAndGlobPath(t *testing.T) {
	rules, err := ParseRules(`!error.type:Skip stack.abs_path:"**/my-utils/*.js" -> my-utils`)
	if err != nil {
		t.Fatal(err)
	}
	hit := Match(rules, Input{
		ExceptionType: "Error",
		Frames:        []Frame{{Filename: "src/my-utils/foo.js", AbsPath: "/app/src/my-utils/foo.js"}},
	})
	if hit == nil {
		t.Fatal("expected path glob match")
	}
	skip := Match(rules, Input{
		ExceptionType: "Skip",
		Frames:        []Frame{{Filename: "src/my-utils/foo.js", AbsPath: "/app/src/my-utils/foo.js"}},
	})
	if skip != nil {
		t.Fatal("negated type should miss")
	}
}

func TestRuleOverridesSDKAndExpandsDefault(t *testing.T) {
	rules, err := ParseRules(`stack.function:query_database -> {{ default }}, {{ transaction }}`)
	if err != nil {
		t.Fatal(err)
	}
	in := Input{
		Config:         Config20260901,
		Rules:          rules,
		SDKFingerprint: []string{"sdk-group"},
		ExceptionType:  "E",
		Transaction:    "/checkout",
		Frames:         []Frame{{Filename: "db.go", Function: "query_database", InApp: true}},
	}
	got := Group(in)
	def := defaultNew(Input{Config: Config20260901, ExceptionType: "E", Frames: in.Frames})
	want := hash(def.Primary + "|/checkout")
	if got.Primary != want || got.Variant != VariantRule {
		t.Fatalf("got %+v want %s", got, want)
	}
}

func TestRuleTitle(t *testing.T) {
	rules, err := ParseRules(`logger:my.package.* level:error -> error-logger, {{ logger }} title="Error from Logger {{ logger }}"`)
	if err != nil {
		t.Fatal(err)
	}
	got := Group(Input{
		Config:  ConfigV1,
		Rules:   rules,
		Logger:  "my.package.foo",
		Level:   "error",
		Message: "x",
	})
	if got.Title != "Error from Logger my.package.foo" {
		t.Fatalf("title=%q", got.Title)
	}
}

func TestUnknownMatcherRejected(t *testing.T) {
	_, err := ParseRules(`family:native -> x`)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTagsMatcher(t *testing.T) {
	rules, err := ParseRules(`tags.server_name:"canary-*" -> canary-events`)
	if err != nil {
		t.Fatal(err)
	}
	hit := Match(rules, Input{Tags: map[string]string{"server_name": "canary-1.internal"}})
	if hit == nil {
		t.Fatal("expected tag match")
	}
}

func TestAppMatcher(t *testing.T) {
	rules, err := ParseRules(`app:yes stack.function:assert -> assertion`)
	if err != nil {
		t.Fatal(err)
	}
	miss := Match(rules, Input{Frames: []Frame{{Function: "assert", InApp: false}}})
	if miss != nil {
		t.Fatal("system assert should miss")
	}
	hit := Match(rules, Input{Frames: []Frame{{Function: "assert", InApp: true}}})
	if hit == nil {
		t.Fatal("in-app assert should hit")
	}
}
