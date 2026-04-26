package sys_config

import "testing"

func TestDecodeSysConfigCacheSyncPayload(t *testing.T) {
	payload, ok := decodeSysConfigCacheSyncPayload(`{"source":"node-1","timestamp":123}`)
	if !ok {
		t.Fatal("expected payload to decode")
	}
	if payload.Source != "node-1" || payload.Timestamp != 123 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestDecodeSysConfigCacheSyncPayloadRejectsInvalidJSON(t *testing.T) {
	_, ok := decodeSysConfigCacheSyncPayload("{invalid")
	if ok {
		t.Fatal("expected invalid payload to be rejected")
	}
}
