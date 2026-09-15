package consultation

import (
	"encoding/json"
	"testing"
)

func TestMarshalDurableMessagePartsUsesCanonicalLowercaseTextShape(t *testing.T) {
	raw, err := marshalDurableMessageParts([]PartInput{{Type: "text", Text: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 || parts[0]["type"] != "text" || parts[0]["text"] != "hello" {
		t.Fatalf("unexpected durable text parts: %s", raw)
	}
	for _, forbidden := range []string{"Type", "Text", "UploadID", "MimeType", "ImageURL"} {
		if _, ok := parts[0][forbidden]; ok {
			t.Fatalf("legacy Go field %q leaked into durable message part: %s", forbidden, raw)
		}
	}
}

func TestMarshalDurableMessagePartsPreservesCanonicalImageIdentity(t *testing.T) {
	raw, err := marshalDurableMessageParts([]PartInput{{
		Type: "image", UploadID: "11111111-1111-4111-8111-111111111111",
		MimeType: "image/png", ImageURL: "/api/v1/uploads/11111111-1111-4111-8111-111111111111/content",
	}})
	if err != nil {
		t.Fatal(err)
	}
	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		t.Fatal(err)
	}
	part := parts[0]
	if part["type"] != "image" || part["upload_id"] != "11111111-1111-4111-8111-111111111111" || part["mime_type"] != "image/png" {
		t.Fatalf("unexpected durable image part: %s", raw)
	}
}

func TestMarshalDurableMessagePartsRejectsUnknownVariant(t *testing.T) {
	if _, err := marshalDurableMessageParts([]PartInput{{Type: "legacy"}}); err == nil {
		t.Fatal("unknown runtime part must fail closed before durable persistence")
	}
}
