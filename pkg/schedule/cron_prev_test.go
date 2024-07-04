package schedule

import (
	"strings"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

/*
These tests are based on https://github.com/robfig/cron/blob/master/spec_test.go
But modified to test the equivalent behaviour for 'prev' parsing
*/

func TestActivation(t *testing.T) {
	tests := []struct {
		time, spec string
		expected   bool
	}{
		// Every fifteen minutes.
		{"Mon Jul 9 15:00 2012", "0/15 * * * *", true},
		{"Mon Jul 9 15:45 2012", "0/15 * * * *", true},
		{"Mon Jul 9 15:40 2012", "0/15 * * * *", false},

		// Every fifteen minutes, starting at 5 minutes.
		{"Mon Jul 9 15:05 2012", "5/15 * * * *", true},
		{"Mon Jul 9 15:20 2012", "5/15 * * * *", true},
		{"Mon Jul 9 15:50 2012", "5/15 * * * *", true},

		// Named months
		{"Sun Jul 15 15:00 2012", "0/15 * * Jul *", true},
		{"Sun Jul 15 15:00 2012", "0/15 * * Jun *", false},

		// Everything set.
		{"Sun Jul 15 08:30 2012", "30 08 ? Jul Sun", true},
		{"Sun Jul 15 08:30 2012", "30 08 15 Jul ?", true},
		{"Mon Jul 16 08:30 2012", "30 08 ? Jul Sun", false},
		{"Mon Jul 16 08:30 2012", "30 08 15 Jul ?", false},

		// Predefined schedules
		{"Mon Jul 9 15:00 2012", "@hourly", true},
		{"Mon Jul 9 15:04 2012", "@hourly", false},
		{"Mon Jul 9 15:00 2012", "@daily", false},
		{"Mon Jul 9 00:00 2012", "@daily", true},
		{"Mon Jul 9 00:00 2012", "@weekly", false},
		{"Sun Jul 8 00:00 2012", "@weekly", true},
		{"Sun Jul 8 01:00 2012", "@weekly", false},
		{"Sun Jul 8 00:00 2012", "@monthly", false},
		{"Sun Jul 1 00:00 2012", "@monthly", true},

		// Test interaction of DOW and DOM.
		// If both are restricted, then only one needs to match.
		{"Sun Jul 15 00:00 2012", "* * 1,15 * Sun", true},
		{"Fri Jun 15 00:00 2012", "* * 1,15 * Sun", true},
		{"Wed Aug 1 00:00 2012", "* * 1,15 * Sun", true},
		{"Sun Jul 15 00:00 2012", "* * */10 * Sun", true}, // verifies #70

		// However, if one has a star, then both need to match.
		{"Sun Jul 15 00:00 2012", "* * * * Mon", false},
		{"Mon Jul 9 00:00 2012", "* * 1,15 * *", false},
		{"Sun Jul 15 00:00 2012", "* * 1,15 * *", true},
		{"Sun Jul 15 00:00 2012", "* * */2 * Sun", true},
	}

	for _, test := range tests {
		sched, err := cron.ParseStandard(test.spec)
		if err != nil {
			t.Error(err)
			continue
		}
		actual := cronPrev(sched.(*cron.SpecSchedule), getTime(test.time).Add(1*time.Second))
		expected := getTime(test.time)
		if test.expected && expected != actual || !test.expected && expected == actual {
			t.Errorf("Fail evaluating %s on %s: (expected) %s != %s (actual)",
				test.spec, test.time, expected, actual)
		}
	}
}

func TestPrev(t *testing.T) {
	runs := []struct {
		time, spec string
		expected   string
	}{
		// Simple cases
		{"Mon Jul 9 15:15 2012", "0 0/15 * * * *", "Mon Jul 9 15:15 2012"}, // for cronPrev we INCLUDE the current time, which is different to cron.Next
		{"Mon Jul 9 15:01 2012", "0 0/15 * * * *", "Mon Jul 9 15:00 2012"},
		{"Mon Jul 9 15:00:01 2012", "0 0/15 * * * *", "Mon Jul 9 15:00 2012"},

		// Wrap around hours
		{"Mon Jul 9 17:05 2012", "0 20-35/15 * * * *", "Mon Jul 9 16:35 2012"},

		// Wrap around days
		{"Tue Jul 10 00:05 2012", "0 */15 * * * *", "Tue Jul 10 00:00 2012"},
		{"Tue Jul 10 00:00 2012", "0 20-35/15 * * * *", "Mon Jul 9 23:35 2012"},
		{"Tue Jul 10 00:00 2012", "15/35 20-35/15 * * * *", "Mon Jul 9 23:35:50 2012"},
		{"Tue Jul 10 00:00 2012", "15/35 20-35/15 1/2 * * *", "Mon Jul 9 23:35:50 2012"},
		{"Tue Jul 10 00:00 2012", "15/35 20-35/15 10-12 * * *", "Mon Jul 9 12:35:50 2012"},

		{"Tue Jul 10 00:00 2012", "15/35 20-35/15 1/2 */2 * *", "Mon Jul 9 23:35:50 2012"},
		{"Tue Jul 10 00:00 2012", "15/35 20-35/15 * 1-8 * *", "Sun Jul 8 23:35:50 2012"},
		{"Tue Jul 10 00:00 2012", "15/35 20-35/15 * 9-20 Jul *", "Mon Jul 9 23:35:50 2012"},

		// Wrap around months
		{"Mon Jul 9 00:00 2012", "0 0 0 10 Apr-Oct ?", "Sun Jun 10 00:00 2012"},
		{"Mon Jul 9 00:00 2012", "0 0 0 */5 Apr,Aug,Oct Mon", "Mon Apr 30 00:00 2012"},

		// Interesting edge case. If both the dom and dow are restricted (not '*') then
		// the schedule should fire if _either_ restrictions match. In the case of
		//   0 0 0 */5 Jan Mon
		// That means midnight on a day in January which is either [1,6,11,16,21,26,31] OR Monday
		// HOWEVER if one of them is unrestricted (exactly '*') then the other must match exactly
		// See https://github.com/robfig/cron/issues/70 for more info
		{"Mon Jul 9 00:00 2012", "0 0 0 */5 Jan Mon", "Mon Jan 31 00:00 2012"}, // The 31st is not a Monday but does match */5
		{"Mon Jul 9 00:00 2012", "0 0 0 */7 Jan Tue", "Tue Jan 31 00:00 2012"}, // The 31st is a Tuesday but is not */7

		// Wrap around years
		{"Mon Jul 9 00:00 2012", "0 0 0 * Oct Tue", "Tue Oct 25 00:00 2011"},
		{"Mon Jul 9 00:00 2012", "0 0 0 * Oct Tue/2", "Sat Oct 29 00:00 2011"},

		// Leap year
		{"Mon Jul 9 23:35 2017", "0 0 0 29 Feb ?", "Mon Feb 29 00:00 2016"},

		// Daylight savings time 2am EST (-5) -> 3am EDT (-4), happened on 11th March 2012: https://www.timeanddate.com/time/dst/2012.html
		// Therefore the most recent 'midnight' before 4am EDT is midnight EST (-5), or 1am (-4)
		{"2012-03-11T04:00:00-0400", "TZ=America/New_York 0 0 0 11 Mar ?", "2012-03-11T00:00:00-0500"},

		// hourly job over the EST -> EDT transition in 2012
		// e.g. at 4am the previous time is 3:05am. At 3am it's 1:05am
		{"2012-03-11T05:00:00-0400", "TZ=America/New_York 0 5 * * * ?", "2012-03-11T04:05:00-0400"},
		{"2012-03-11T04:00:00-0400", "TZ=America/New_York 0 5 * * * ?", "2012-03-11T03:05:00-0400"},
		{"2012-03-11T03:00:00-0400", "TZ=America/New_York 0 5 * * * ?", "2012-03-11T01:05:00-0500"},
		{"2012-03-11T01:00:00-0500", "TZ=America/New_York 0 5 * * * ?", "2012-03-11T00:05:00-0500"},
		{"2012-03-11T00:00:00-0500", "TZ=America/New_York 0 5 * * * ?", "2012-03-10T23:05:00-0500"},

		// // hourly job using CRON_TZ
		{"2012-03-11T05:00:00-0400", "CRON_TZ=America/New_York 0 5 * * * ?", "2012-03-11T04:05:00-0400"},
		{"2012-03-11T04:00:00-0400", "CRON_TZ=America/New_York 0 5 * * * ?", "2012-03-11T03:05:00-0400"},
		{"2012-03-11T03:00:00-0400", "CRON_TZ=America/New_York 0 5 * * * ?", "2012-03-11T01:05:00-0500"},
		{"2012-03-11T01:00:00-0500", "CRON_TZ=America/New_York 0 5 * * * ?", "2012-03-11T00:05:00-0500"},
		{"2012-03-11T00:00:00-0500", "CRON_TZ=America/New_York 0 5 * * * ?", "2012-03-10T23:05:00-0500"},

		// 1am nightly job
		{"2012-03-12T00:00:00-0400", "TZ=America/New_York 0 0 1 * * ?", "2012-03-11T01:00:00-0500"},
		{"2012-03-11T00:00:00-0500", "TZ=America/New_York 0 0 1 * * ?", "2012-03-10T01:00:00-0500"},

		// 2am nightly job (skipped)
		{"2012-03-11T03:00:00-0400", "TZ=America/New_York 0 0 2 * * ?", "2012-03-10T02:00:00-0500"},

		// Daylight savings time 2am EDT (-4) => 1am EST (-5), happened on 4th November 2012
		{"2012-11-04T03:00:00-0500", "TZ=America/New_York 0 30 2 04 Nov ?", "2012-11-04T02:30:00-0500"},
		{"2012-11-04T01:15:00-0500", "TZ=America/New_York 0 30 1 04 Nov ?", "2012-11-04T01:30:00-0400"},

		// hourly job
		{"2012-11-04T03:00:00-0500", "TZ=America/New_York 0 5 * * * ?", "2012-11-04T02:05:00-0500"},
		{"2012-11-04T02:00:00-0500", "TZ=America/New_York 0 5 * * * ?", "2012-11-04T01:05:00-0500"},
		{"2012-11-04T01:00:00-0500", "TZ=America/New_York 0 5 * * * ?", "2012-11-04T01:05:00-0400"},

		// 1am nightly job (runs twice)
		{"2012-11-04T01:00:00-0400", "TZ=America/New_York 0 5 1 * * ?", "2012-11-03T01:05:00-0400"},
		{"2012-11-04T01:00:00-0500", "TZ=America/New_York 0 5 1 * * ?", "2012-11-04T01:05:00-0400"},
		{"2012-11-05T01:00:00-0500", "TZ=America/New_York 0 5 1 * * ?", "2012-11-04T01:05:00-0500"},

		// 2am nightly job
		{"2012-11-04T03:00:00-0500", "TZ=America/New_York 0 0 2 * * ?", "2012-11-04T02:00:00-0500"},
		{"2012-11-05T01:00:00-0500", "TZ=America/New_York 0 0 2 * * ?", "2012-11-04T02:00:00-0500"},

		// hourly job
		{"TZ=America/New_York 2012-11-04T01:00:00-0400", "0 5 * * * ?", "2012-11-04T00:05:00-0400"},
		{"TZ=America/New_York 2012-11-04T02:00:00-0400", "0 5 * * * ?", "2012-11-04T01:05:00-0400"},
		{"TZ=America/New_York 2012-11-04T03:00:00-0500", "0 5 * * * ?", "2012-11-04T02:05:00-0500"},

		// 1am nightly job (runs twice)
		{"TZ=America/New_York 2012-11-04T00:05:00-0400", "0 0 1 * * ?", "2012-11-03T01:00:00-0400"},
		{"TZ=America/New_York 2012-11-04T01:05:00-0400", "0 0 1 * * ?", "2012-11-04T01:00:00-0400"},
		{"TZ=America/New_York 2012-11-04T01:05:00-0500", "0 0 1 * * ?", "2012-11-04T01:00:00-0500"},

		// 2am nightly job
		{"TZ=America/New_York 2012-11-04T03:00:00-0500", "0 5 2 * * ?", "2012-11-04T02:05:00-0500"},
		{"TZ=America/New_York 2012-11-04T02:00:00-0500", "0 5 2 * * ?", "2012-11-03T02:05:00-0400"},

		// Unsatisfiable
		{"Mon Jul 9 23:35 2012", "0 0 0 30 Feb ?", ""},
		{"Mon Jul 9 23:35 2012", "0 0 0 31 Apr ?", ""},

		// Monthly job
		{"TZ=America/New_York 2012-12-02T00:00:00-0500", "0 0 3 3 * ?", "2012-11-03T03:00:00-0400"},

		// Test the scenario of DST resulting in midnight not being a valid time.
		// https://github.com/robfig/cron/issues/157 For example: Sao Paulo has DST that transforms midnight on
		// 11/3 into 1am
		{"2018-11-09T05:00:00-0400", "TZ=America/Sao_Paulo 0 0 9 10 * ?", "2018-10-10T08:00:00-0400"},
		{"2018-02-23T05:00:00-0500", "TZ=America/Sao_Paulo 0 0 9 22 * ?", "2018-02-22T07:00:00-0500"},
	}

	var secondParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.DowOptional | cron.Descriptor)

	for _, c := range runs {
		sched, err := secondParser.Parse(c.spec)
		if err != nil {
			t.Error(err)
			continue
		}
		actual := cronPrev(sched.(*cron.SpecSchedule), getTime(c.time))
		expected := getTime(c.expected)
		if !actual.Equal(expected) {
			t.Errorf("%s, \"%s\": (expected) %v != %v (actual). %+v. %v", c.time, c.spec, expected, actual, sched, actual.Weekday())
		}
	}
}

func getTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}

	var location = time.Local
	if strings.HasPrefix(value, "TZ=") {
		parts := strings.Fields(value)
		loc, err := time.LoadLocation(parts[0][len("TZ="):])
		if err != nil {
			panic("could not parse location:" + err.Error())
		}
		location = loc
		value = parts[1]
	}

	var layouts = []string{
		"Mon Jan 2 15:04 2006",
		"Mon Jan 2 15:04:05 2006",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, location); err == nil {
			return t
		}
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05-0700", value, location); err == nil {
		return t
	}
	panic("could not parse time value " + value)
}

func getTimeTZ(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse("Mon Jan 2 15:04 2006", value)
	if err != nil {
		t, err = time.Parse("Mon Jan 2 15:04:05 2006", value)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05-0700", value)
			if err != nil {
				panic(err)
			}
		}
	}

	return t
}
