package datorium

import "testing"

func TestWriteResultDistributionComplete(t *testing.T) {
	complete, err := DecodeResult([]byte(`{
		"ok": true,
		"command": "create",
		"collection": "Movies",
		"id": "01TESTMOVIES00000000000001",
		"$": "Movies:0",
		"#": "01VER",
		"operationId": "01OP",
		"distributionComplete": true
	}`))
	if err != nil {
		t.Fatal(err)
	}
	wr := writeResultFrom(complete)
	if !wr.DistributionComplete {
		t.Fatalf("expected DistributionComplete true: %+v", wr)
	}
	if wr.Note != nil {
		t.Fatalf("expected nil Note: %+v", wr.Note)
	}

	incomplete, err := DecodeResult([]byte(`{
		"ok": true,
		"command": "patch",
		"collection": "Movies",
		"id": "01TESTMOVIES00000000000001",
		"operationId": "01OP2",
		"distributionComplete": false,
		"versions": {"before": "01A", "after": "01B"},
		"note": {
			"code": "replication_retry_scheduled",
			"message": "Write succeeded on the SOT-member, but one or more read members did not acknowledge within the timeout.",
			"required": ["serverB", "serverD"],
			"acknowledged": ["serverB"],
			"unacknowledged": ["serverD"],
			"timeoutMs": 10000
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	wr = writeResultFrom(incomplete)
	if wr.DistributionComplete {
		t.Fatalf("expected DistributionComplete false: %+v", wr)
	}
	if wr.Version != "01B" || wr.VersionBefore != "01A" {
		t.Fatalf("versions: before=%q after=%q", wr.VersionBefore, wr.Version)
	}
	if wr.Note == nil {
		t.Fatal("expected Note")
	}
	if wr.Note.Code != "replication_retry_scheduled" {
		t.Fatalf("note code %q", wr.Note.Code)
	}
	if wr.Note.TimeoutMs != 10000 {
		t.Fatalf("timeoutMs %d", wr.Note.TimeoutMs)
	}
	if len(wr.Note.Required) != 2 || wr.Note.Required[0] != "serverB" {
		t.Fatalf("required %#v", wr.Note.Required)
	}
	if len(wr.Note.Acknowledged) != 1 || wr.Note.Acknowledged[0] != "serverB" {
		t.Fatalf("acknowledged %#v", wr.Note.Acknowledged)
	}
	if len(wr.Note.Unacknowledged) != 1 || wr.Note.Unacknowledged[0] != "serverD" {
		t.Fatalf("unacknowledged %#v", wr.Note.Unacknowledged)
	}
}
