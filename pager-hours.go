package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"io"
	"fmt"
	"github.com/discordianfish/pager-hours/holidays"
	"github.com/discordianfish/pager-hours/pagerduty"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"
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

var (
	month      = beginningOfMonth(time.Now())
	token      = flag.String("token", "", "PagerDuty token.")
	domain     = flag.String("domain", "", "PagerDuty subdomain/organization.")
	from       = flag.String("from", month.AddDate(0, -1, 0).Format(shortDate), "Calculate hours after this date.")
	to         = flag.String("to", month.Format(shortDate), "Calculate hours before this date.")
	policyId   = flag.String("policy", "", "Escalation policy to get on call hours and incidents from")
	csvHeaders = []string{
		"Date",
		"User",
		"Time Zone",
		"Location",
		"Type",
		"Hours On-Call",
		"Hours with Incidents/Day",
		"Hours with Incidents/Night",
		"Additional Hours/Day",
		"Additional Hours/Night",
	}
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

type pagerHours struct {
	officeTZ  map[string]holidays.Region
	incidents map[string]map[int][]pagerduty.Incident
	entries   []pagerduty.ScheduleEntries
	pd        pagerduty.Client
}

func New(officeTZ map[string]holidays.Region) *pagerHours {
	return &pagerHours{
		officeTZ: officeTZ,
		pd:       pagerduty.New(*domain, *token),
	}
}

func (p *pagerHours) getHours(policyId string, from, to time.Time) error {
	policy, err := p.pd.GetEscalationPolicy(policyId)
	if err != nil {
		return fmt.Errorf("Couldn't get escalation policy: %s", err)
	}
	log.Printf("Calculating hours for %s between %s and %s", policy.Name, from, to)
	schedule := policy.Rules[0].Object // TODO: Verify this is sorted right
	log.Printf("- Using schedule %s", schedule.Name)

	serviceIds := []string{}
	for _, service := range policy.Services {
		log.Printf("-- service %s", service.Name)
		serviceIds = append(serviceIds, service.Id)
	}

	log.Println("- Getting all incidents for services")
	incidents, err := p.pd.GetIncidents(from, to, serviceIds)
	if err != nil {
		return fmt.Errorf("Couldn't get incidents: %s", err)
	}

	incidentMap := map[string]map[int][]pagerduty.Incident{}
	for _, incident := range *incidents {
		if incident.EscalationPolicy.Id != policyId {
			continue
		}
		c := incident.CreatedOn
		if _, ok := incidentMap[c.Format(shortDate)]; !ok {
			incidentMap[c.Format(shortDate)] = map[int][]pagerduty.Incident{}
		}
		incidentMap[c.Format(shortDate)][c.Hour()] = append(incidentMap[c.Format(shortDate)][c.Hour()], incident)
	}

	log.Println("- Getting entries for schedule")
	entries, err := p.pd.GetScheduleEntries(schedule.Id, from, to)
	if err != nil {
		return fmt.Errorf("Couldn't get schedule entries for %s: %s", schedule.Name, err)
	}
	p.entries = entries
	p.incidents = incidentMap
	return nil
}

func (p *pagerHours) writeFile(file io.ReadWriter) {
	workers := make(map[string]worker)

	csvw := csv.NewWriter(file)
	csvw.Write(csvHeaders)

	day := map[worker]map[string]workload{}
	// work := map[worker]map[string]map[string]int{}

	for _, entry := range p.entries {
		current := entry.Start
		for current.Before(entry.End) {
			email := entry.User.Email
			if _, ok := workers[email]; !ok {
				workers[email] = p.getUser(entry.User.Id)
			}

			user := workers[email]
			if _, ok := day[user]; !ok {
				day[user] = map[string]workload{}
			}

			currentLocal := current.In(user.location) // local time for the user working that hour
			bucket := bucketFor(currentLocal, user)

			work := day[user][bucket]
			work.oncall++

			incidents := p.incidents[current.Format(shortDate)][current.Hour()]

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

func (p *pagerHours) listEscalationPolicies() {
	policies, err := p.pd.GetEscalationPolicies()
	if err != nil {
		log.Fatalf("Couldn't get policies: %s", err)
	}

	for _, policy := range *policies {
		fmt.Printf("- %s %s\n", policy.Id, policy.Name)
	}
}

func (p *pagerHours) getUser(id string) worker {
	puser, err := p.pd.GetUser(id)
	if err != nil {
		log.Fatalf("Couldn't get user %s: %s", id, err)
	}
	region, ok := p.officeTZ[puser.TimeZone]
	if !ok {
		log.Fatalf("No office in %s known", puser.TimeZone)
	}

	return worker{
		email:    puser.Email,
		location: puser.Location,
		region:   region,
	}
}

func main() {
	officeTZ := map[string]holidays.Region{
		"Berlin": holidays.Berlin,
		"Sofia":  holidays.Bulgaria,
		"Pacific Time (US & Canada)": holidays.California,
	}

	p := New(officeTZ)
	if *policyId == "" {
		fmt.Println("No policy (-policy=abc) specified, available policies:")
		p.listEscalationPolicies()
		os.Exit(0)
	}


	fromTime, err := time.Parse(shortDate, *from)
	if err != nil {
		log.Fatalf("Please provide a valid start date (format: %s)", shortDate)
	}

	toTime, err := time.Parse(shortDate, *to)
	if err != nil {
		log.Fatalf("Please provide a valid end date (format: %s)", shortDate)
	}

	if err := p.getHours(*policyId, fromTime, toTime); err != nil {
		log.Fatalf("Couldn't get hours for policy %s: ", err)
	}

	file := &bytes.Buffer{}
	p.writeFile(file)

	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("Couldn't read generated csv: %s", err)
	}
	fmt.Printf("%s", content)
}
