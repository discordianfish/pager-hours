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
	token  = flag.String("token", "", "PagerDuty token.")
	domain = flag.String("domain", "", "PagerDuty subdomain/organization.")
	since  = flag.Int("since", -1, "Calculate hours since X month ago.")
	workers  map[string]worker
	officeTZ map[string]holidays.Region
	matchTier = regexp.MustCompile("tier=([0-9]*)")
)

const (
	weekday     = "weekday"
	saturday    = "saturday"
	sunday      = "sunday"
	holiday     = "holiday"
	office      = "officehours"
	officeStart = 10
	officeEnd   = 18
)

type workload struct {
	tier  int
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
	if *since >= 0 {
		log.Fatalf("Dude, I can't look into the future! -since takes negativ numbers only.")
	}
	workers = make(map[string]worker)
	officeTZ = map[string]holidays.Region{
		"Berlin": holidays.Berlin,
		"Sofia": holidays.Bulgaria,
		"Pacific Time (US & Canada)": holidays.California,
	}
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

	if t.Hour() > officeStart && t.Hour() < officeEnd {
		return office
	}
	return weekday
}

func main() {
	pd := pagerduty.New(*domain, *token)
	now := time.Now()
	schedules, err := pd.GetSchedules()
	if err != nil {
		log.Fatalf("Couldn't get Schedules: %s", err)
	}

	for _, schedule := range schedules {
		matches := matchTier.FindStringSubmatch(schedule.Name)
		if len(matches) != 2 { // 0: matched string, 1: capture
			continue
		}
		log.Printf("Using %s.", schedule.Name)
		tier, err := strconv.Atoi(matches[1])
		if err != nil {
			log.Printf("Skipping %s because tier '%s' is not a number.", schedule.Name, matches[1])
		}
		entries, err := pd.GetScheduleEntries(schedule.Id, now.AddDate(0, *since, 0), now)
		if err != nil {
			log.Fatalf("Couldn't get schedule entries for %s: %s", schedule.Id, err)
		}
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
						email:    entry.User.Email,
						location: time.FixedZone(puser.TimeZone, int(puser.GmtOffset.Seconds())),
						region:   region,
						workload: make(map[string]workload),
					}
				}

				user := workers[entry.User.Email];
				currentLocal := current.In(user.location) // local time for the user working that hour
				bucket := bucketFor(currentLocal, user)
				if _, ok := user.workload[bucket]; !ok {
					user.workload[bucket] = workload{tier: tier}
				}
				user.workload[bucket] = workload{tier: tier, hours: workers[entry.User.Email].workload[bucket].hours + 1}
				current = current.Add(1 * time.Hour)
			}
		}
	}
	csvw := csv.NewWriter(os.Stdout)
	for _, worker := range workers {
		for bucket, workload := range worker.workload {
			csvw.Write([]string{worker.email, worker.location.String(), string(worker.region), strconv.Itoa(workload.tier), bucket, strconv.Itoa(workload.hours)})
		}
	}
	csvw.Flush()
}
