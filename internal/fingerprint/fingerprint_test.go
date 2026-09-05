package fingerprint

import "testing"

func TestV1MatchesLegacyHash(t *testing.T) {
	frames := []Frame{{Filename: "app/foo.go", Function: "run", InApp: true}}
	got := Compute(nil, "Error", "boom 123", frames)
	raw := "Error|app/foo.go:run|" + normalizeMessage("boom 123")
	want := hash(raw)
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestV1ExplicitFingerprint(t *testing.T) {
	a := Compute([]string{"demo-group"}, "Error", "message A", nil)
	b := Compute([]string{"demo-group"}, "Error", "message B", nil)
	if a != b {
		t.Fatal("explicit fingerprint should ignore message")
	}
	def := Compute(nil, "Error", "message A", nil)
	if a == def {
		t.Fatal("explicit fingerprint should differ from default")
	}
}

func TestDefaultPlaceholderFallsThrough(t *testing.T) {
	frames := []Frame{{Filename: "a.go", Function: "f", InApp: true}}
	with := Group(Input{Config: ConfigV1, SDKFingerprint: []string{"{{ default }}"}, ExceptionType: "E", Message: "m", Frames: frames})
	without := Group(Input{Config: ConfigV1, ExceptionType: "E", Message: "m", Frames: frames})
	if with.Primary != without.Primary || with.Variant != VariantV1 {
		t.Fatalf("got %+v want %+v", with, without)
	}
}

func TestDefaultPlaceholderMixed(t *testing.T) {
	frames := []Frame{{Filename: "a.go", Function: "f", InApp: true}}
	in := Input{Config: ConfigV1, SDKFingerprint: []string{"mine", "{{ default }}"}, ExceptionType: "E", Message: "m", Frames: frames}
	got := Group(in)
	def := defaultV1(in)
	want := hash("mine|" + def.Primary)
	if got.Primary != want || got.Variant != VariantCustom {
		t.Fatalf("got %+v want hash %s", got, want)
	}
}

func TestNewConfigIgnoresMessageWhenStackPresent(t *testing.T) {
	frames := []Frame{
		{Filename: "lib.js", Function: "lib", InApp: false},
		{Filename: "app.js", Function: "handle", InApp: true},
	}
	a := Group(Input{Config: Config20260901, ExceptionType: "TypeError", Message: "one", Frames: frames})
	b := Group(Input{Config: Config20260901, ExceptionType: "TypeError", Message: "two", Frames: frames})
	if a.Primary != b.Primary {
		t.Fatal("same in-app stack should group despite different messages")
	}
	if a.Variant != VariantApp {
		t.Fatalf("variant=%s", a.Variant)
	}
	if len(a.Variants) != 2 {
		t.Fatalf("expected app+system variants, got %+v", a.Variants)
	}
}

func TestNewConfigExceptionFallback(t *testing.T) {
	a := Group(Input{Config: Config20260901, ExceptionType: "TypeError", Message: "no stack here"})
	b := Group(Input{Config: Config20260901, ExceptionType: "TypeError", Message: "different"})
	if a.Primary == b.Primary {
		t.Fatal("exception grouping should include normalized value")
	}
	if a.Variant != VariantException {
		t.Fatalf("variant=%s", a.Variant)
	}
}

func TestNewConfigMessageFallback(t *testing.T) {
	a := Group(Input{Config: Config20260901, Logger: "app", Message: "hello 99"})
	b := Group(Input{Config: Config20260901, Logger: "app", Message: "hello 12"})
	if a.Primary != b.Primary {
		t.Fatal("normalized messages should group")
	}
	if a.Variant != VariantMessage {
		t.Fatalf("variant=%s", a.Variant)
	}
}

func TestTopInAppFramePreference(t *testing.T) {
	frames := []Frame{
		{Filename: "boot.go", Function: "main", InApp: false},
		{Filename: "app.go", Function: "Run", InApp: true},
		{Filename: "lib.go", Function: "Call", InApp: false},
	}
	f := topFrame(frames)
	if f == nil || f.Function != "Run" {
		t.Fatalf("top in-app = %+v", f)
	}
}

func TestStackKeyUsesAllInAppFrames(t *testing.T) {
	frames := []Frame{
		{Filename: "a.go", Function: "A", InApp: true},
		{Filename: "b.go", Function: "B", InApp: true},
	}
	same := Group(Input{Config: Config20260901, ExceptionType: "E", Frames: frames})
	diff := Group(Input{Config: Config20260901, ExceptionType: "E", Frames: frames[:1]})
	if same.Primary == diff.Primary {
		t.Fatal("full in-app stack should differ from a single frame")
	}
}
