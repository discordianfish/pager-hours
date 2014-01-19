package holidays

import (
	"errors"
	"fmt"
	"time"
)

type Region string

const (
	Berlin     Region = "Berlin"
	Bulgaria   Region = "Bulgaria"
	California Region = "California"
	NewYork    Region = "New York"
)

var (
	NoHoliday      = errors.New("No holiday")
	orthodoxEaster = map[int]time.Time{
		2013: time.Date(2013, 5, 5, 0, 0, 0, 0, time.UTC),
		2014: time.Date(2013, 4, 20, 0, 0, 0, 0, time.UTC),
		2015: time.Date(2013, 4, 12, 0, 0, 0, 0, time.UTC),
		2016: time.Date(2013, 5, 1, 0, 0, 0, 0, time.UTC),
		2017: time.Date(2013, 4, 16, 0, 0, 0, 0, time.UTC),
		2018: time.Date(2013, 4, 8, 0, 0, 0, 0, time.UTC),
	}
)

type holiday struct {
	Name string
}

func Holiday(t time.Time, r Region) (holiday, error) {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	switch r {
	case Berlin:
		return holidayBerlin(day)
	case Bulgaria:
		return holidayBulgaria(day)
	case California:
		return holidayUSA(day)
	case NewYork:
		return holidayUSA(day)
	}
	return holiday{}, errors.New("Region not supported")
}

func holidayBerlin(t time.Time) (holiday, error) {
	// fixed
	if t.Day() == 1 && t.Month() == time.January {
		return holiday{Name: "New Year's Day"}, nil
	}

	if t.Day() == 1 && t.Month() == time.May {
		return holiday{Name: "Labour Day"}, nil
	}

	if t.Day() == 3 && t.Month() == time.October {
		return holiday{Name: "German Unity Day"}, nil
	}

	if t.Day() == 25 && t.Month() == time.December {
		return holiday{Name: "Christmas Day"}, nil
	}

	if t.Day() == 26 && t.Month() == time.December {
		return holiday{Name: "St. Stephen's Day"}, nil
	}

	// dynamic
	eastern := Easter(t)

	if t.Equal(eastern) {
		return holiday{Name: "Easter"}, nil
	}

	if t.Equal(eastern.AddDate(0, 0, -2)) {
		return holiday{Name: "Good Friday"}, nil
	}

	if t.Equal(eastern.AddDate(0, 0, 1)) {
		return holiday{Name: "Easter Monday"}, nil
	}

	if t.Equal(eastern.AddDate(0, 0, 39)) {
		return holiday{Name: "Ascension Day"}, nil
	}

	if t.Equal(eastern.AddDate(0, 0, 50)) {
		return holiday{Name: "Whit Monday"}, nil
	}

	return holiday{}, NoHoliday
}

func holidayUSA(t time.Time) (holiday, error) {
	// US-wide
	if t.Day() == 1 && t.Month() == time.January {
		return holiday{Name: "New Year's Day"}, nil
	}

	if t.Day() == 4 && t.Month() == time.July {
		return holiday{Name: "Independence Day"}, nil
	}

	if t.Day() == 25 && t.Month() == time.December {
		return holiday{Name: "Christmas Day"}, nil
	}

	// US-wide/dynamic
	if t.Weekday() == time.Monday && t.Month() == time.September && nthDay(t) == 1 {
		return holiday{Name: "Labor Day"}, nil
	}

	if t.Weekday() == time.Thursday && t.Month() == time.November && nthDay(t) == 4 {
		return holiday{Name: "Thanksgiving Day"}, nil
	}
	if t.Weekday() == time.Friday && t.Month() == time.November && nthDay(t.AddDate(0, 0, -1)) == 4 { // "Friday after 4th Thursday in November"
		return holiday{Name: "Day after Thanksgiving"}, nil
	}

	if t.Weekday() == time.Monday && t.Month() == time.May && nthDayRev(t) == 1 {
		return holiday{Name: "Memorial Day"}, nil
	}

	if t.Weekday() == time.Monday && t.Month() == time.January && nthDay(t) == 3 {
		return holiday{Name: "Martin Luther King Jr. Day"}, nil
	}

	return holiday{}, NoHoliday
}

func holidayBulgaria(t time.Time) (holiday, error) {
	easter, ok := orthodoxEaster[t.Year()]
	if !ok {
		return holiday{}, fmt.Errorf("Don't know orthodox easter for year %d", t.Year())
	}

	if t.Equal(easter) {
		return holiday{Name: "Easter"}, nil
	}

	if t.Equal(easter.AddDate(0, 0, -2)) {
		return holiday{Name: "Good Friday"}, nil
	}

	if t.Equal(easter.AddDate(0, 0, -1)) {
		return holiday{Name: "Easter Saturday"}, nil
	}

	if t.Equal(easter.AddDate(0, 0, 1)) {
		return holiday{Name: "Easter Monday"}, nil
	}

	if t.Day() == 1 && t.Month() == time.January {
		return holiday{Name: "New Year's Day"}, nil
	}

	if t.Day() == 2 && t.Month() == time.January {
		return holiday{Name: "Day after New Year's Day"}, nil
	}

	if t.Day() == 3 && t.Month() == time.March {
		return holiday{Name: "Liberation Day"}, nil
	}

	if t.Day() == 1 && t.Month() == time.May {
		return holiday{Name: "Labour Day"}, nil
	}

	if t.Day() == 6 && t.Month() == time.May {
		return holiday{Name: "St. George's Day"}, nil
	}

	if t.Day() == 24 && t.Month() == time.May {
		return holiday{Name: "Bulgarian Education and Culture and Slavonic Literature Day"}, nil
	}

	if t.Day() == 6 && t.Month() == time.September {
		return holiday{Name: "Unification Day"}, nil
	}

	if t.Day() == 22 && t.Month() == time.September {
		return holiday{Name: "Independence Day"}, nil
	}

	if t.Day() == 1 && t.Month() == time.November {
		return holiday{Name: "Day of the Bulgarian Enlighteners"}, nil
	}

	if t.Day() == 24 && t.Month() == time.December {
		return holiday{Name: "Christmas Eve"}, nil
	}

	if t.Day() == 25 && t.Month() == time.December {
		return holiday{Name: "Christmas Day"}, nil
	}

	if t.Day() == 26 && t.Month() == time.December {
		return holiday{Name: "Second Day of Christmas"}, nil
	}

	return holiday{}, NoHoliday
}

//returns the number of the weekday in a month (is it the 1th, 2th.. Monday)
func nthDay(t time.Time) (n int) {
	cDay := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())

	for cDay.Before(t) || cDay.Equal(t) {
		if cDay.Weekday() == t.Weekday() {
			n++
		}
		cDay = cDay.AddDate(0, 0, 1)
	}
	return
}

func nthDayRev(t time.Time) (n int) {
	cDay := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location()).AddDate(0, 0, -1) // last day in Month

	for cDay.After(t) || cDay.Equal(t) {
		if cDay.Weekday() == t.Weekday() {
			n++
		}
		cDay = cDay.AddDate(0, 0, -1)
	}
	return
}

// -- http://rosettacode.org/wiki/Holidays_related_to_Easter#Goa
func mod(a, n int) int {
	r := a % n
	if r < 0 {
		return r + n
	}
	return r
}

func Easter(t time.Time) time.Time {
	y := t.Year()
	c := y / 100
	n := mod(y, 19)
	i := mod(c-c/4-(c-(c-17)/25)/3+19*n+15, 30)
	i -= (i / 28) * (1 - (i/28)*(29/(i+1))*((21-n)/11))
	l := i - mod(y+y/4+i+2-c+c/4, 7)
	m := 3 + (l+40)/44
	d := l + 28 - 31*(m/4)
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, t.Location())
}

// --
