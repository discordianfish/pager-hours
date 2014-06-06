pager-hours
===========

For a given escalation policy, this tool exports:
- for the main schedule, how many hours each user was on call
- for services using that escalation policy _primarly_, how many incidents each user received (see known issues)
- exports this data to google drive

If you want to compensate on call duty, you often need to track hours on call during the weekday outside office hours, weekends and holidays.
This tool helps with that.

## Usage

- Add an API client in pagerduty

## Sum via Google Spreadsheet

        =QUERY('2013-05'!A:F, "select B, E, sum(F) group by B, E")

## Known issues
This tool has a lot of limitations and assumptions.
- PagerDuty has no concept of "Location" for a user beside their time zone, therefor we map a pagerduty timezone to a office location (see holidays.Region)
- This list is still hardcoded in init() like this:

        officeTZ = map[string]holidays.Region{
          "Berlin": holidays.Berlin,
          "Sofia": holidays.Bulgaria,
          "Pacific Time (US & Canada)": holidays.California,
        }

- The underlying libraries (holidays and pagerduty) are very limited and only support what we're using here
- Therefor generalizing this tool makes only sense after supporting more regions in the holidays library.
- The incident count reflects how many incidents a user received and ignoring whether the incident was escalated and actually handled by someone else.
