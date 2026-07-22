package datorium_test

import (
	"os"
	"path/filepath"
	"testing"

	datorium "github.com/JohnAD/datorium-client-go"
)

func TestDecodeContractFixtures(t *testing.T) {
	root := filepath.Join("testdata", "contract")
	okBody, err := os.ReadFile(filepath.Join(root, "create_ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := datorium.DecodeResult(okBody)
	if err != nil || !res.OK {
		t.Fatalf("create_ok: %#v err=%v", res, err)
	}
	if res.StringField("id") == "" || res.StringField("#") == "" {
		t.Fatalf("missing fields: %#v", res.Env)
	}

	wmBody, err := os.ReadFile(filepath.Join(root, "wrong_machine.json"))
	if err != nil {
		t.Fatal(err)
	}
	res, err = datorium.DecodeResult(wmBody)
	if err != nil || res.OK {
		t.Fatalf("wrong_machine: %#v err=%v", res, err)
	}
	if res.FirstErrorCode() != datorium.CodeWrongMachine {
		t.Fatalf("code %q", res.FirstErrorCode())
	}
	if res.StringField("correctServer") != "server2" {
		t.Fatalf("correctServer %q", res.StringField("correctServer"))
	}
}
