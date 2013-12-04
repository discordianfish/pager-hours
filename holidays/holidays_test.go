package holidays_test

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/soundcloud/pager-hours/holidays"
)

func TestHolidayAllBerlin(t *testing.T) {
	if err := compareToFixtures(holidays.Berlin, "test/fixtures/holidays_berlin.csv"); err != nil {
		t.Logf("Failed: %s", err)
		t.FailNow()
	}
}

func TestHolidayAllCalifornia(t *testing.T) {
	if err := compareToFixtures(holidays.California, "test/fixtures/holidays_california.csv"); err != nil {
		t.Logf("Failed: %s", err)
		t.FailNow()
	}
}

func TestHolidayAllBulgaria(t *testing.T) {
	if err := compareToFixtures(holidays.Bulgaria, "test/fixtures/holidays_bulgaria.csv"); err != nil {
		t.Logf("Failed: %s", err)
		t.FailNow()
	}
}

func compareToFixtures(region holidays.Region, file string) error {
	fd, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("Couldn't open fixtures '%s': %s", file, err)
	}
	csv := csv.NewReader(fd)

	for {
		day, err := csv.Read()
		if err == io.EOF {
			break
		}
		dt, err := time.Parse("2006-01-02", day[0])
		if err != nil {
			return fmt.Errorf("Couldn't parse fixtures: %s", err)
		}
		t := dt.Add(5 * time.Hour)
		holiday, err := holidays.Holiday(t, region)
		if err != nil {
			return fmt.Errorf("%s is supposed to be a holiday but library disagrees", t)
		}

		if holiday.Name != day[1] {
			return fmt.Errorf("Holiday/library: %s, fixture: %s", holiday.Name, day[1])
		}
	}
	return nil
}

// the ryan case
func TestMemorialDayPST(t *testing.T) {
	dt, err := time.Parse("2006-01-02 15:04:05 -0700", "2013-05-20 16:00:00 -0800")
	if err != nil {
		t.Logf("Couldn't parse timestamp: %s", err)
		t.FailNow()
	}
	holiday, err := holidays.Holiday(dt, holidays.California)
	if err == nil {
		t.Logf("%s isn't a holiday but library says it's %s", dt, holiday)
		t.FailNow()
	}
}
