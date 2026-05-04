package cli

import (
	"reflect"
	"testing"
)

func TestNormalizeUIs(t *testing.T) {
	got := normalizeUIs([]string{" docs ", "swagger", "Scalar", "", "redoc"})
	want := []string{"swagger", "scalar", "redoc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeUIs = %#v, want %#v", got, want)
	}
}
