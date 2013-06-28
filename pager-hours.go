package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"github.com/discordianfish/pager-hours/holidays"
	"github.com/discordianfish/pager-hours/pagerduty"
	"log"
	"os"
	"strconv"
	"time"
)

var (
	month    = beginningOfMonth(time.Now())
	token    = flag.String("token", "", "PagerDuty token.")
	domain   = flag.String("domain", "", "PagerDuty subdomain/organization.")
	from     = flag.String("from", month.AddDate(0, -1, 0).Format(shortDate), "Calculate hours after this date.")
	to       = flag.String("to", month.Format(shortDate), "Calculate hours before this date.")
	policyId = flag.String("policy", "", "Escalation policy to get on call hours and incidents from")
	fromTime time.Time
	toTime   time.Time
	officeTZ map[string]holidays.Region

	csvHeaders = []string{
		"Date",
		"User",
		"Time Zone",
		"Location",
		"Type",
		"Hours On-Call",
		"Incidents/Day",
		"Incidents/Night",
		"Additional Hours/Day",
		"Additional Hours/Night",
	}
)

const (
	shortDate = "2006-01-02"

	weekday  = "weekday"
	saturday = "saturday"
	sunday   = "sunday"
	holiday  = "holiday"

	office      = "officehours"
	officeStart = 10
	officeEnd   = 18

	day        = "day"
	night      = "night"
	nightStart = 0
	nightEnd   = 8
)

type worker struct {
	email    string
	location *time.Location
	region   holidays.Region
}

type workload struct {
	oncall         int
	incidents      int
	incidentsNight int
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
	if *policyId == "" {
		fmt.Println("No policy (-policy=abc) specified, available policies:")
		listEscalationPolicies(pd)
		os.Exit(0)
	}

	policy, err := pd.GetEscalationPolicy(*policyId)
	if err != nil {
		log.Fatalf("Couldn't get escalation policy: %s", err)
	}
	log.Printf("Calculating hours for %s between %s and %s", policy.Name, *from, *to)
	schedule := policy.Rules[0].Object // TODO: Verify this is sorted right
	log.Printf("- Using schedule %s", schedule.Name)

	serviceIds := []string{}
	for _, service := range policy.Services {
		log.Printf("-- service %s", service.Name)
		serviceIds = append(serviceIds, service.Id)
	}

	log.Println("- Getting all incidents for services")
	incidents, err := pd.GetIncidents(fromTime, toTime, serviceIds)
	if err != nil {
		log.Fatalf("Couldn't get incidents: %s", err)
	}

	incidentMap := map[string]map[int][]pagerduty.Incident{}
	for _, incident := range *incidents {
		if incident.EscalationPolicy.Id != *policyId {
			continue
		}
		c := incident.CreatedOn
		if _, ok := incidentMap[c.Format(shortDate)]; !ok {
			incidentMap[c.Format(shortDate)] = map[int][]pagerduty.Incident{}
		}
		incidentMap[c.Format(shortDate)][c.Hour()] = append(incidentMap[c.Format(shortDate)][c.Hour()], incident)
	}

	log.Println("- Getting entries for schedule")
	entries, err := pd.GetScheduleEntries(schedule.Id, fromTime, toTime)
	if err != nil {
		log.Fatalf("Couldn't get schedule entries for %s: %s", schedule.Name, err)
	}

	workers := make(map[string]worker)
	csvw := csv.NewWriter(os.Stdout)
	csvw.Write(csvHeaders)

	day := map[worker]map[string]workload{}
	// work := map[worker]map[string]map[string]int{}

	for _, entry := range entries {
		current := entry.Start
		for current.Before(entry.End) {
			email := entry.User.Email
			if _, ok := workers[email]; !ok {
				workers[email] = getUser(pd, entry.User.Id)
			}

			user := workers[email]
			if _, ok := day[user]; !ok {
				day[user] = map[string]workload{}
			}

			currentLocal := current.In(user.location) // local time for the user working that hour
			bucket := bucketFor(currentLocal, user)

			work := day[user][bucket]
			work.oncall++

			incidents := incidentMap[current.Format(shortDate)][current.Hour()]

			if len(incidents) > 0 {
				if current.Hour() >= nightStart && current.Hour() < nightEnd {
					work.incidentsNight++
				} else {
					work.incidents++
				}
			}
			day[user][bucket] = work

			next := current.Add(1 * time.Hour)
			if next.Day() != current.Day() {
				for user, buckets := range day {
					for bucket, work := range buckets {
						if work.oncall == 0 && work.incidents == 0 && work.incidentsNight == 0 {
							continue
						}
						csvw.Write([]string{
							current.Format(shortDate),
							user.email,
							user.location.String(),
							string(user.region),
							bucket,
							strconv.Itoa(work.oncall),
							strconv.Itoa(work.incidents),
							strconv.Itoa(work.incidentsNight),
							"0", "0",
						})
						csvw.Flush()
						day[user][bucket] = workload{}
					}
				}
			}
			current = next
		}
	}
}

func getUser(pd pagerduty.PagerDuty, id string) worker {
	puser, err := pd.GetUser(id)
	if err != nil {
		log.Fatalf("Couldn't get user %s: %s", id, err)
	}
	region, ok := officeTZ[puser.TimeZone]
	if !ok {
		log.Fatalf("No office in %s known", puser.TimeZone)
	}

	return worker{
		email:    puser.Email,
		location: puser.Location,
		region:   region,
	}
}

func listEscalationPolicies(pd pagerduty.PagerDuty) {
	policies, err := pd.GetEscalationPolicies()
	if err != nil {
		log.Fatalf("Couldn't get policies: %s", err)
	}

	for _, policy := range *policies {
		fmt.Printf("- %s %s\n", policy.Id, policy.Name)
	}
}
