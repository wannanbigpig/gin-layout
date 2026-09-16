package config

import (
	"errors"
	"reflect"
	"testing"
)

func TestConfigReloadDiscardsCandidatesWhenLaterPreparationFails(t *testing.T) {
	old := GetConfig()
	t.Cleanup(func() { setActiveConfig(old) })
	next := cloneDefaultConfig()
	var events []string
	handlers := []ConfigReloadHandler{
		{Name: "resource", Prepare: func(*Conf, *Conf, ConfigDiff) (ConfigReloadChange, error) {
			return ConfigReloadChange{Commit: func() { events = append(events, "commit") }, Abort: func() { events = append(events, "abort") }, Finalize: func() { events = append(events, "close-old") }}, nil
		}},
		{Name: "later", Handle: func(*Conf, *Conf, ConfigDiff) error { return errors.New("candidate unavailable") }},
	}
	if err := applyConfigReload(old, next, ConfigDiff{}, handlers); err == nil {
		t.Fatal("expected preparation failure")
	}
	if GetConfig() != old || !reflect.DeepEqual(events, []string{"abort"}) {
		t.Fatalf("failed reload changed live state: %v", events)
	}
}

func TestConfigReloadPublishesBeforeClosingPreviousResources(t *testing.T) {
	old := GetConfig()
	t.Cleanup(func() { setActiveConfig(old) })
	next := cloneDefaultConfig()
	committed, finalized := false, false
	handlers := []ConfigReloadHandler{{Name: "resource", Prepare: func(*Conf, *Conf, ConfigDiff) (ConfigReloadChange, error) {
		return ConfigReloadChange{Commit: func() { committed = true }, Abort: func() { t.Error("successful reload aborted") }, Finalize: func() {
			finalized = true
			if !committed || GetConfig() != next {
				t.Error("old resource closed before publishing")
			}
		}}, nil
	}}}
	if err := applyConfigReload(old, next, ConfigDiff{}, handlers); err != nil {
		t.Fatal(err)
	}
	if !finalized {
		t.Fatal("old resources not finalized")
	}
}
