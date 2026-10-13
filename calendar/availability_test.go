package calendar

import (
	"testing"
	"time"
)

func availabilityPtr(a Availability) *Availability { return &a }

func TestMarshalCreateInput_Availability(t *testing.T) {
	base := func() CreateEventInput {
		return CreateEventInput{Title: "x", StartDate: time.Now(), EndDate: time.Now().Add(time.Hour)}
	}
	t.Run("omitted when nil", func(t *testing.T) {
		b, err := marshalCreateInput(base())
		if err != nil {
			t.Fatal(err)
		}
		if contains(string(b), "availability") {
			t.Errorf("availability key should be omitted: %s", b)
		}
	})
	t.Run("free serialized", func(t *testing.T) {
		in := base()
		in.Availability = availabilityPtr(AvailabilityFree)
		b, err := marshalCreateInput(in)
		if err != nil {
			t.Fatal(err)
		}
		if !contains(string(b), `"availability":1`) {
			t.Errorf("expected availability 1: %s", b)
		}
	})
	t.Run("busy (zero value) still sent when pointer set", func(t *testing.T) {
		in := base()
		in.Availability = availabilityPtr(AvailabilityBusy)
		b, err := marshalCreateInput(in)
		if err != nil {
			t.Fatal(err)
		}
		if !contains(string(b), `"availability":0`) {
			t.Errorf("expected availability 0: %s", b)
		}
	})
	t.Run("notSupported rejected", func(t *testing.T) {
		in := base()
		in.Availability = availabilityPtr(AvailabilityNotSupported)
		if _, err := marshalCreateInput(in); err == nil {
			t.Error("expected error for AvailabilityNotSupported")
		}
	})
	t.Run("out of range rejected", func(t *testing.T) {
		in := base()
		in.Availability = availabilityPtr(Availability(7))
		if _, err := marshalCreateInput(in); err == nil {
			t.Error("expected error for out-of-range availability")
		}
	})
}

func TestMarshalUpdateInput_Availability(t *testing.T) {
	t.Run("omitted when nil", func(t *testing.T) {
		b, err := marshalUpdateInput(UpdateEventInput{})
		if err != nil {
			t.Fatal(err)
		}
		if contains(string(b), "availability") {
			t.Errorf("availability key should be omitted: %s", b)
		}
	})
	for _, tc := range []struct {
		in   Availability
		want string
	}{
		{AvailabilityBusy, `"availability":0`},
		{AvailabilityFree, `"availability":1`},
		{AvailabilityTentative, `"availability":2`},
		{AvailabilityUnavailable, `"availability":3`},
	} {
		t.Run(tc.in.String(), func(t *testing.T) {
			b, err := marshalUpdateInput(UpdateEventInput{Availability: availabilityPtr(tc.in)})
			if err != nil {
				t.Fatal(err)
			}
			if !contains(string(b), tc.want) {
				t.Errorf("expected %s: %s", tc.want, b)
			}
		})
	}
	t.Run("notSupported rejected", func(t *testing.T) {
		if _, err := marshalUpdateInput(UpdateEventInput{Availability: availabilityPtr(AvailabilityNotSupported)}); err == nil {
			t.Error("expected error for AvailabilityNotSupported")
		}
	})
}
