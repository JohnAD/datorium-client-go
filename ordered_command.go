package datorium

import (
	"fmt"

	"github.com/JohnAD/ojson"
)

// BuildCommandOrdered formats a four-field JSON command request whose detail object
// is serialized with ojson field order preserved. detail must be an object
// (or Void/missing, treated as {}).
func BuildCommandOrdered(word, target, parm string, detail ojson.JSONValue) ([]byte, error) {
	if word == "" || target == "" || parm == "" {
		return nil, fmt.Errorf("datorium: word, target, and parm are required")
	}
	if detail.IsMissing() {
		detail = ojson.NewObject()
	}
	if !detail.IsObject() {
		return nil, fmt.Errorf("datorium: detail must be a JSON object, got %s", detail.Kind())
	}
	raw := detail.ToJSONBytes()
	if len(raw) == 0 || raw[0] != '{' {
		return nil, fmt.Errorf("datorium: detail must be a JSON object")
	}
	return marshalCommandRequest(word, target, parm, raw)
}

func ensureOperationIDValue(detail ojson.JSONValue) {
	if !detail.IsObject() {
		return
	}
	if !detail.HasField("operationId") {
		detail.Set("operationId", ojson.NewString(NewOperationID()))
	}
}
