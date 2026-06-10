package taskcenter

import "testing"

func TestDecodeTaskDefinitionChangePayload(t *testing.T) {
	payload, ok := DecodeTaskDefinitionChangePayload(`{"source":"node-1","task_code":"cron:demo","timestamp":123}`)
	if !ok {
		t.Fatal("expected payload to decode")
	}
	if payload.Source != "node-1" || payload.TaskCode != "cron:demo" || payload.Timestamp != 123 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestDecodeTaskDefinitionChangePayloadRejectsInvalidJSON(t *testing.T) {
	_, ok := DecodeTaskDefinitionChangePayload("{invalid")
	if ok {
		t.Fatal("expected invalid payload to be rejected")
	}
}
