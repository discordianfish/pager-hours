package pagerduty

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	apiUrl     = "https://%s.pagerduty.com/api/v1"
	dateLayout = "02-01-2006"
)

type Common struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

type Schedule struct {
	Today    string `json:"today"`
	TimeZone string `json:"time_zone"`
	Name     string `json:"name"`
	Id       string `json:"id"`
}

type Schedules struct {
	Schedules []Schedule `json:"schedules"`
	Common
}

type ScheduleDetails struct {
	Schedule struct {
		FinalSchedule struct {
			ScheduleEntries []ScheduleEntries `json:"rendered_schedule_entries"`
		} `json:"final_schedule"`
	} `json:"schedule"`
}

type ScheduleEntries struct {
	User  UserDetails `json:"user"`
	End   time.Time   `json:"end"`
	Start time.Time   `json:"start"`
}

type User struct {
	User UserDetails `json:"user"`
}

type UserDetails struct {
	Id       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	TimeZone string `json:"time_zone"`
	Location *time.Location
}

type Incidents struct {
	Common
	Incidents []Incident `json:"incidents"`
}

type Incident struct {
	CreatedOn           time.Time         `json:"created_on"`
	NumberOfEscalations int               `json:"number_of_escalations"`
	TriggerSummaryData  map[string]string `json:"trigger_summary_data"`
	EscalationPolicy    struct {
		Name string `json:"name"`
		Id   string `json:"id"`
	} `json:"escalation_policy"`
}

type PagerDuty struct {
	token string
	url   string
}

func New(domain string, token string) (pd PagerDuty) {
	pd.token = token
	pd.url = fmt.Sprintf(apiUrl, domain)

	return pd
}

//FIXME: pagination
func (pd *PagerDuty) getBody(path string, params url.Values) (body []byte, err error) {
	url := fmt.Sprintf("%s/%s?%s", pd.url, path, params.Encode())
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/json")
	req.Header.Set("Authorization", fmt.Sprintf("Token token=%s", pd.token))
	resp, err := client.Do(req)
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return body, fmt.Errorf("Status %s != 200", resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return body, fmt.Errorf("getBody: %s: %s", url, err)
	}

	return body, err
}

func (pd *PagerDuty) GetUser(id string) (UserDetails, error) {
	body, err := pd.getBody(fmt.Sprintf("users/%s", id), url.Values{})
	if err != nil {
		return UserDetails{}, fmt.Errorf("Couldn't request user: %s", err)
	}

	var pdu User
	if err := json.Unmarshal(body, &pdu); err != nil {
		return UserDetails{}, fmt.Errorf("Couldn't unmarshal response: %s", err)
	}
	user := pdu.User
	locationName, ok := ianaLocation[user.TimeZone]
	if !ok {
		return UserDetails{}, fmt.Errorf("Timezone %s couldn't be mapped to a location", user.TimeZone)
	}
	location, err := time.LoadLocation(locationName)
	if err != nil {
		return UserDetails{}, fmt.Errorf("Location %s couldn't be loaded", locationName)
	}
	user.Location = location

	return user, nil
}

func (pd *PagerDuty) GetSchedules() ([]Schedule, error) {
	body, err := pd.getBody("schedules", url.Values{})
	if err != nil {
		return []Schedule{}, fmt.Errorf("Couldn't request schedules: %s", err)
	}

	var pds Schedules
	if err := json.Unmarshal(body, &pds); err != nil {
		return []Schedule{}, fmt.Errorf("Couldn't unmarshal response: %s", err)
	}
	if pds.Total >= pds.Limit {
		return pds.Schedules, fmt.Errorf("Pagination not yet supported but necessary since total entries (%d) > limit (%d)", pds.Total, pds.Limit)
	}
	return pds.Schedules, err
}

func (pd *PagerDuty) GetScheduleEntries(id string, since time.Time, until time.Time) ([]ScheduleEntries, error) {
	params := url.Values{}
	params.Set("since", since.Format(dateLayout))
	params.Set("until", until.Format(dateLayout))

	body, err := pd.getBody(fmt.Sprintf("schedules/%s", id), params)
	if err != nil {
		return []ScheduleEntries{}, fmt.Errorf("Couldn't request schedule/%s: %s", id, err)
	}

	var pdsd ScheduleDetails
	if err := json.Unmarshal(body, &pdsd); err != nil {
		return []ScheduleEntries{}, fmt.Errorf("Couldn't unmarshal response: %s", err)
	}

	return pdsd.Schedule.FinalSchedule.ScheduleEntries, err
}

func (pd *PagerDuty) GetIncidents(since time.Time, until time.Time) (*[]Incident, error) {
	incidents := []Incident{}
	offset := 0
	limit := 100
	total := limit + 1 // make sure the for condition is true

	params := url.Values{}
	params.Set("since", since.Format(dateLayout))
	params.Set("until", until.Format(dateLayout))
	params.Set("limit", strconv.Itoa(limit))

	for offset+limit <= total {
		// log.Printf("offset: %d, limit: %d, total: %d", offset, limit, total)
		params.Set("offset", strconv.Itoa(offset))
		body, err := pd.getBody("incidents", params)
		if err != nil {
			return nil, fmt.Errorf("Couldn't request incidents: %s", err)
		}

		incs := Incidents{}
		if err := json.Unmarshal(body, &incs); err != nil {
			return nil, fmt.Errorf("Couldn't unmarshal response: %s", err)
		}
		incidents = append(incidents, incs.Incidents...)
		offset = incs.Offset + limit
		total = incs.Total
	}
	return &incidents, nil
}
