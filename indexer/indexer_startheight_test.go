package indexer

import "testing"

func TestComputeStartHeight_Unset(t *testing.T) {
	dbNext := int64(100)
	chainHead := int64(150)
	got := computeStartHeight(dbNext, chainHead, false, false, 0)
	if got != dbNext {
		t.Fatalf("expected %d, got %d", dbNext, got)
	}
}

func TestComputeStartHeight_Latest(t *testing.T) {
	dbNext := int64(100)
	chainHead := int64(150)
	got := computeStartHeight(dbNext, chainHead, true, true, 0)
	if got != chainHead {
		t.Fatalf("expected %d, got %d", chainHead, got)
	}
}

func TestComputeStartHeight_ExplicitBelowDB(t *testing.T) {
	dbNext := int64(200)
	chainHead := int64(500)
	got := computeStartHeight(dbNext, chainHead, true, false, 150)
	if got != dbNext {
		t.Fatalf("expected clamp to dbNext %d, got %d", dbNext, got)
	}
}

func TestComputeStartHeight_ExplicitAboveHead(t *testing.T) {
	dbNext := int64(0)
	chainHead := int64(123)
	got := computeStartHeight(dbNext, chainHead, true, false, 999)
	if got != chainHead {
		t.Fatalf("expected clamp to head %d, got %d", chainHead, got)
	}
}

func TestComputeStartHeight_ExplicitWithinRange(t *testing.T) {
	dbNext := int64(10)
	chainHead := int64(100)
	desired := int64(42)
	got := computeStartHeight(dbNext, chainHead, true, false, desired)
	if got != desired {
		t.Fatalf("expected %d, got %d", desired, got)
	}
}

func TestComputeStartHeight_NegativeClampedToZeroThenDB(t *testing.T) {
	dbNext := int64(5)
	chainHead := int64(100)
	got := computeStartHeight(dbNext, chainHead, true, false, -10)
	if got != dbNext {
		t.Fatalf("expected clamp to dbNext %d for negative start, got %d", dbNext, got)
	}
}
