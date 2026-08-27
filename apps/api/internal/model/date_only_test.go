package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDateOnlyJSONRoundTrip(t *testing.T) {
	var date DateOnly
	if err := json.Unmarshal([]byte(`"1998-05-20"`), &date); err != nil {
		t.Fatalf("unmarshal date: %v", err)
	}
	if got := date.String(); got != "1998-05-20" {
		t.Fatalf("date string = %q, want 1998-05-20", got)
	}

	encoded, err := json.Marshal(date)
	if err != nil {
		t.Fatalf("marshal date: %v", err)
	}
	if got := string(encoded); got != `"1998-05-20"` {
		t.Fatalf("encoded date = %s", got)
	}
}

func TestDateOnlyAgeAt(t *testing.T) {
	date, err := ParseDateOnly("1998-09-20")
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	if got := date.AgeAt(time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)); got != 27 {
		t.Fatalf("age before birthday = %d, want 27", got)
	}
	if got := date.AgeAt(time.Date(2026, time.September, 20, 12, 0, 0, 0, time.UTC)); got != 28 {
		t.Fatalf("age on birthday = %d, want 28", got)
	}
}

func TestDateOnlyRejectsNonDateJSON(t *testing.T) {
	var date DateOnly
	if err := json.Unmarshal([]byte(`"1998/05/20"`), &date); err == nil {
		t.Fatal("expected invalid date format to fail")
	}
}

func TestDateOnlyScansDatabaseTimeWithoutAddingTimeToJSON(t *testing.T) {
	var date DateOnly
	if err := date.Scan(time.Date(1998, time.May, 20, 15, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))); err != nil {
		t.Fatalf("scan database date: %v", err)
	}
	if got := date.String(); got != "1998-05-20" {
		t.Fatalf("date string = %q, want 1998-05-20", got)
	}
}
