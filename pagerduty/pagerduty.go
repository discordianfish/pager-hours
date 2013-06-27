package pagerduty

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

const (
	apiUrl     = "https://%s.pagerduty.com/api/v1"
	dateLayout = "02-01-2006"
)

type pdCommon struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

type pdSchedule struct {
	Today    string `json:"today"`
	TimeZone string `json:"time_zone"`
	Name     string `json:"name"`
	Id       string `json:"id"`
}

type pdSchedules struct {
	Schedules []pdSchedule `json:"schedules"`
	pdCommon
}

type pdScheduleDetails struct {
	Schedule struct {
		FinalSchedule struct {
			ScheduleEntries []pdScheduleEntries `json:"rendered_schedule_entries"`
		} `json:"final_schedule"`
	} `json:"schedule"`
}

type pdScheduleEntries struct {
	User  pdUserDetails `json:"user"`
	End   time.Time     `json:"end"`
	Start time.Time     `json:"start"`
}

type pdUser struct {
	User pdUserDetails `json:"user"`
}

type pdUserDetails struct {
	Id        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	TimeZone  string `json:"time_zone"`
	Location  *time.Location
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

func (pd *PagerDuty) GetUser(id string) (pdUserDetails, error) {
	body, err := pd.getBody(fmt.Sprintf("users/%s", id), url.Values{})
	if err != nil {
		return pdUserDetails{}, fmt.Errorf("Couldn't request user: %s", err)
	}

	var pdu pdUser
	if err := json.Unmarshal(body, &pdu); err != nil {
		return pdUserDetails{}, fmt.Errorf("Couldn't unmarshal response: %s", err)
	}
	user := pdu.User
	locationName, ok := ianaLocation[user.TimeZone]
	if !ok {
		return pdUserDetails{}, fmt.Errorf("Timezone %s couldn't be mapped to a location", user.TimeZone)
	}
	location, err := time.LoadLocation(locationName)
	if err != nil {
		return pdUserDetails{}, fmt.Errorf("Location %s couldn't be loaded", locationName)
	}
	user.Location = location

	return user, nil
}

func (pd *PagerDuty) GetSchedules() ([]pdSchedule, error) {
	body, err := pd.getBody("schedules", url.Values{})
	if err != nil {
		return []pdSchedule{}, fmt.Errorf("Couldn't request schedules: %s", err)
	}

	var pds pdSchedules
	if err := json.Unmarshal(body, &pds); err != nil {
		return []pdSchedule{}, fmt.Errorf("Couldn't unmarshal response: %s", err)
	}
	if pds.Total >= pds.Limit {
		return pds.Schedules, fmt.Errorf("Pagination not yet supported but necessary since total entries (%d) > limit (%d)", pds.Total, pds.Limit)
	}
	return pds.Schedules, err
}

func (pd *PagerDuty) GetScheduleEntries(id string, since time.Time, until time.Time) ([]pdScheduleEntries, error) {
	params := url.Values{}
	params.Set("since", since.Format(dateLayout))
	params.Set("until", until.Format(dateLayout))

	body, err := pd.getBody(fmt.Sprintf("schedules/%s", id), params)
	if err != nil {
		return []pdScheduleEntries{}, fmt.Errorf("Couldn't request schedule/%s: %s", id, err)
	}

	var pdsd pdScheduleDetails
	if err := json.Unmarshal(body, &pdsd); err != nil {
		return []pdScheduleEntries{}, fmt.Errorf("Couldn't unmarshal response: %s", err)
	}

	return pdsd.Schedule.FinalSchedule.ScheduleEntries, err
}
