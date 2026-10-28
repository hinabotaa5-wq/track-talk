package httpx

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// DB が無いとき(nil)は、二段階認証を調べずに通す。
func TestMFAEnrollmentNilPool(t *testing.T) {
	var m *MFAEnrollment
	if ok, err := m.Enrolled(context.Background(), uuid.New()); ok || err != nil {
		t.Fatalf("nil %v %v", ok, err)
	}
	if ok, err := NewMFAEnrollment(nil).Enrolled(context.Background(), uuid.New()); ok || err != nil {
		t.Fatalf("nil pool %v %v", ok, err)
	}
}
