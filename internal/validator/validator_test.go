package validator

import (
	"sync"
	"testing"
)

func TestInitValidatorTransCanRetryAfterFailure(t *testing.T) {
	resetValidatorRuntimeForTest()
	t.Cleanup(resetValidatorRuntimeForTest)

	if err := InitValidatorTrans("invalid-locale"); err == nil {
		t.Fatal("expected invalid locale initialization to fail")
	}

	resetValidatorRuntimeForTest()

	if err := InitValidatorTrans("zh"); err != nil {
		t.Fatalf("expected retry with zh locale to succeed, got %v", err)
	}
}

func resetValidatorRuntimeForTest() {
	validatorRuntime = newValidatorRuntime()
	regexCache = sync.Map{}
}
