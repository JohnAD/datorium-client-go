package datorium

import (
	"fmt"

	"github.com/JohnAD/ojson"
)

// BuildCommandOrdered formats an access-language command whose detail object
// is serialized with ojson field order preserved. detail must be an object
// (or Void/missing, treated as {}).
func BuildCommandOrdered(word, target, parm string, detail ojson.JSONValue) (string, error) {
	if word == "" || target == "" || parm == "" {
		return "", fmt.Errorf("datorium: word, target, and parm are required")
	}
	if detail.IsMissing() {
		detail = ojson.NewObject()
	}
	if !detail.IsObject() {
		return "", fmt.Errorf("datorium: detail must be a JSON object, got %s", detail.Kind())
	}
	raw := detail.ToJSONBytes()
	return fmt.Sprintf("%s %s %s %s", word, target, parm, string(raw)), nil
}

func ensureOperationIDValue(detail ojson.JSONValue) {
	if !detail.IsObject() {
		return
	}
	if !detail.HasField("operationId") {
		detail.Set("operationId", ojson.NewString(NewOperationID()))
	}
}
