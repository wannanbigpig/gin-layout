package validator

import (
	"sync"
	"testing"
)

func TestInitValidatorTransCanRetryAfterFailure(t *testing.T) {
	resetValidatorRuntimeForTest()
	t.Cleanup(resetValidatorRuntimeForTest)

	if err := InitValidatorTrans("invalid-locale"); err != nil {
		t.Fatalf("expected invalid locale to fallback successfully, got %v", err)
	}
	if validatorRuntime.translatorLocale != "zh" {
		t.Fatalf("expected invalid locale to fallback to zh, got %q", validatorRuntime.translatorLocale)
	}

	resetValidatorRuntimeForTest()

	if err := InitValidatorTrans("en"); err != nil {
		t.Fatalf("expected english locale initialization to succeed, got %v", err)
	}
	if validatorRuntime.translatorLocale != "en" {
		t.Fatalf("expected default translator locale en, got %q", validatorRuntime.translatorLocale)
	}
}

func resetValidatorRuntimeForTest() {
	validatorRuntime = newValidatorRuntime()
	regexCache = sync.Map{}
}
