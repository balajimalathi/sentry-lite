package process

import "testing"

func TestNormalizeExtractsTransactionAndThreadFrames(t *testing.T) {
	payload := []byte(`{
		"event_id": "abc",
		"transaction": "/checkout",
		"message": "boom",
		"threads": {
			"values": [{
				"crashed": true,
				"stacktrace": {
					"frames": [
						{"filename": "app.js", "function": "handle", "in_app": true}
					]
				}
			}]
		}
	}`)
	n, err := Normalize(payload)
	if err != nil {
		t.Fatal(err)
	}
	if n.Transaction != "/checkout" {
		t.Fatalf("transaction=%q", n.Transaction)
	}
	if len(n.Frames) != 1 || n.Frames[0].Function != "handle" {
		t.Fatalf("frames=%+v", n.Frames)
	}
}

func TestNormalizePrefersExceptionStackOverThreads(t *testing.T) {
	payload := []byte(`{
		"exception": {"values": [{
			"type": "Error",
			"value": "x",
			"stacktrace": {"frames": [{"filename": "a.go", "function": "A", "in_app": true}]}
		}]},
		"threads": {"values": [{
			"crashed": true,
			"stacktrace": {"frames": [{"filename": "b.go", "function": "B", "in_app": true}]}
		}]}
	}`)
	n, err := Normalize(payload)
	if err != nil {
		t.Fatal(err)
	}
	if n.ExceptionType != "Error" || len(n.Frames) != 1 || n.Frames[0].Function != "A" {
		t.Fatalf("%+v", n)
	}
}
