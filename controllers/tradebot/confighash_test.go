package tradebot

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestComputeConfigHash(t *testing.T) {
	hashA := computeConfigHash(map[string]string{"config.json": `{"a":1}`}, "class A")
	hashB := computeConfigHash(map[string]string{"config.json": `{"a":2}`}, "class A")
	hashAAgain := computeConfigHash(map[string]string{"config.json": `{"a":1}`}, "class A")

	if len(hashA) != 16 {
		t.Errorf("expected a 16-character hash, got %q (%d chars)", hashA, len(hashA))
	}
	if hashA == hashB {
		t.Error("expected different config.json content to produce a different hash")
	}
	if hashA != hashAAgain {
		t.Error("expected the same inputs to produce the same hash (must be deterministic)")
	}

	hashDifferentScript := computeConfigHash(map[string]string{"config.json": `{"a":1}`}, "class B")
	if hashA == hashDifferentScript {
		t.Error("expected a different strategy script to also change the hash")
	}
}

func TestConfigDriftCondition(t *testing.T) {
	t.Run("no drift", func(t *testing.T) {
		cond := configDriftCondition("my-bot", "trading", 4, false)
		if cond.Status != metav1.ConditionFalse {
			t.Errorf("expected status False, got %v", cond.Status)
		}
		if cond.Message != "" {
			t.Errorf("expected an empty message when there's no drift, got %q", cond.Message)
		}
	})

	t.Run("drift", func(t *testing.T) {
		cond := configDriftCondition("my-bot", "trading", 4, true)
		if cond.Status != metav1.ConditionTrue {
			t.Errorf("expected status True, got %v", cond.Status)
		}
		if cond.ObservedGeneration != 4 {
			t.Errorf("expected observedGeneration 4, got %d", cond.ObservedGeneration)
		}
		if !strings.Contains(cond.Message, "kubectl rollout restart statefulset/my-bot -n trading") {
			t.Errorf("expected the message to name the kubectl remedy, got %q", cond.Message)
		}
		if !strings.Contains(cond.Message, "updateStrategy: Auto") {
			t.Errorf("expected the message to name the Auto-mode remedy, got %q", cond.Message)
		}
	})
}
