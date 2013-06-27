package main

import (
	"encoding/csv"
	"flag"
	"github.com/discordianfish/pager-hours/holidays"
	"github.com/discordianfish/pager-hours/pagerduty"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"
)

var (
	month     = beginningOfMonth(time.Now())
	token     = flag.String("token", "", "PagerDuty token.")
	domain    = flag.String("domain", "", "PagerDuty subdomain/organization.")
	from      = flag.String("from", month.AddDate(0, -1, 0).Format(shortDate), "Calculate hours after this date.")
	to        = flag.String("to", month.Format(shortDate), "Calculate hours before this date.")
	schedule  = flag.String("schedule", "", "List schedules.")
	fromTime  time.Time
	toTime    time.Time
	officeTZ  map[string]holidays.Region
	matchTier = regexp.MustCompile("tier=([0-9]*)")
)

const (
	shortDate   = "2006-01-02"
	weekday     = "weekday"
	saturday    = "saturday"
	sunday      = "sunday"
	holiday     = "holiday"
	office      = "officehours"
	officeStart = 10
	officeEnd   = 18
)

type workload struct {
	hours int
}

type worker struct {
	email    string
	workload map[string]workload
	location *time.Location
	region   holidays.Region
}

func init() {
	flag.Parse()

	if *token == "" || *domain == "" {
		log.Fatalf("pager-hours -token=<your-token> -domain=<subdomain/organization>")
	}

	var err error
	fromTime, err = time.Parse(shortDate, *from)
	if err != nil {
		log.Fatalf("Please provide a valid start date (format: %s)", shortDate)
	}
	toTime, err = time.Parse(shortDate, *to)
	if err != nil {
		log.Fatalf("Please provide a valid end date (format: %s)", shortDate)
	}

	officeTZ = map[string]holidays.Region{
		"Berlin": holidays.Berlin,
		"Sofia":  holidays.Bulgaria,
		"Pacific Time (US & Canada)": holidays.California,
	}
}

func beginningOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func bucketFor(t time.Time, user worker) string {
	if t.Weekday() == time.Sunday {
		return sunday
	}

	_, err := holidays.Holiday(t, user.region)
	if err == nil {
		return holiday
	}

	if t.Weekday() == time.Saturday {
		return saturday
	}

	if t.Hour() >= officeStart && t.Hour() < officeEnd {
		return office
	}
	return weekday
}

func main() {
	pd := pagerduty.New(*domain, *token)

	log.Printf("Calculating pagerduty hours between %s and %s", *from, *to)

	if *schedule == "" {
		listSchedules(pd)
		os.Exit(0)
	}

	entries, err := pd.GetScheduleEntries(*schedule, fromTime, toTime)
	if err != nil {
		log.Fatalf("Couldn't get schedule entries for %s: %s", *schedule, err)
	}

	workers := make(map[string]worker)
	csvw := csv.NewWriter(os.Stdout)
	csvw.Write([]string{
		"Date",
		"User",
		"Time Zone",
		"Location",
		"Type",
		"Hours",
	})
	for _, entry := range entries {
		current := entry.Start
		for current.Before(entry.End) {
			if _, ok := workers[entry.User.Email]; !ok {
				puser, err := pd.GetUser(entry.User.Id)
				if err != nil {
					log.Fatalf("Couldn't get user %s: %s", entry.User.Id, err)
				}
				region, ok := officeTZ[puser.TimeZone]
				if !ok {
					log.Fatalf("No office in %s known", puser.TimeZone)
				}

				workers[entry.User.Email] = worker{
					email:    puser.Email,
					location: puser.Location,
					region:   region,
					workload: make(map[string]workload),
				}
			}

			user := workers[entry.User.Email]
			currentLocal := current.In(user.location) // local time for the user working that hour
			bucket := bucketFor(currentLocal, user)

			if _, ok := user.workload[bucket]; !ok {
				user.workload[bucket] = workload{}
			}
			user.workload[bucket] = workload{hours: workers[entry.User.Email].workload[bucket].hours + 1}

			next := current.Add(1 * time.Hour)

			if next.Day() != current.Day() {
				for _, worker := range workers {
					for bucket, workload := range worker.workload {
						csvw.Write([]string{
							current.Format(shortDate),
							worker.email,
							worker.location.String(),
							string(worker.region),
							bucket,
							strconv.Itoa(workload.hours),
						})
						csvw.Flush()
						delete(worker.workload, bucket)
					}
				}
			}
			current = next
		}
	}
}

func listSchedules(pd pagerduty.PagerDuty) {
	schedules, err := pd.GetSchedules()
	if err != nil {
		log.Fatalf("Couldn't get Schedules: %s", err)
	}

	for _, schedule := range schedules {
		log.Printf("%s: %s", schedule.Id, schedule.Name)
	}
}
