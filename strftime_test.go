package autolog

import (
	"fmt"
	"testing"
	"time"
)

func TestStrftime(t *testing.T) {
	type testCase struct {
		Time    time.Time
		Pattern string
		Expect  string
	}

	z0 := time.FixedZone("MST", -7*60*60)
	t0 := time.Unix(1136239445, 999999999).In(z0) // 2006-01-02T15:04:05.999999999-0700

	z1 := time.FixedZone("PDT", -7*60*60)
	t1 := time.Unix(1696952439, 111111111).In(z1) // 2023-10-10T08:40:39.111111111-0700

	testData := [...]testCase{
		{t0, "%a, %d %b %Y %H:%M:%S %Z%z", "Mon, 02 Jan 2006 15:04:05 MST-0700"},
		{t1, "%a, %d %b %Y %H:%M:%S %Z%z", "Tue, 10 Oct 2023 08:40:39 PDT-0700"},
		{t0, "%A", "Monday"},
		{t0, "%.3A", "Mon"},
		{t0, "%5.3A", "  Mon"},
		{t0, "%_5.3A", "__Mon"},
		{t0, "%k", "15"},
		{t0, "%l", " 3"},
		{t1, "%k", " 8"},
		{t1, "%l", " 8"},
		{t0, "%C", "20"},
		{t0, "%y", "06"},
	}

	for _, row := range testData {
		name := fmt.Sprintf("[%s][%s]", row.Time.Format(time.RFC3339Nano), row.Pattern)
		t.Run(name, func(t *testing.T) {
			actual := Strftime(row.Pattern, row.Time)
			if actual != row.Expect {
				t.Errorf("wrong result:\n\texpect: %q\n\tactual: %q", row.Expect, actual)
			}
		})
	}
}
