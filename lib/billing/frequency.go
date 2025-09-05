// lib/billing/frequency.go
package billing

import (
	"fmt"
	"log"
	"time"

	"electricity-invoice-calculator/lib/utils"
)

type BillingFrequency string

const (
	Monthly   BillingFrequency = "monthly"
	Quarterly BillingFrequency = "quarterly"
)

type Period struct {
	Start     time.Time
	End       time.Time
	Label     string
	Frequency BillingFrequency
}

// Asks user to choose billing frequency
func GetBillingFrequency() BillingFrequency {
	options := []string{"Monthly", "Quarterly"}
	choice := utils.GetSimpleChoice("How are you billed?", options)

	if choice == 0 {
		return Monthly
	}
	return Quarterly
}

// Returns the first complete quarter after startDate
func GetFirstCompleteQuarter(startDate time.Time) time.Time {
	year := startDate.Year()

	quarters := []time.Time{
		time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(year, 4, 1, 0, 0, 0, 0, time.UTC),
		time.Date(year, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(year, 10, 1, 0, 0, 0, 0, time.UTC),
	}

	for _, quarter := range quarters {
		if quarter.After(startDate) {
			return quarter
		}
	}

	// If not quarter this year, return next year. Though, next year will not be valid for billing.
	return time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)
}

// Returns the first complete month after startDate
func GetFirstCompleteMonth(startDate time.Time) time.Time {
	year, month, _ := startDate.Date()
	nextMonth := month + 1
	nextYear := year

	if nextMonth > 12 {
		nextMonth = 1
		nextYear = year + 1
	}

	return time.Date(nextYear, nextMonth, 1, 0, 0, 0, 0, time.UTC)
}

// Returns the last complete period
func GetLastCompletePeriod(frequency BillingFrequency) (time.Time, error) {
	now := time.Now()

	switch frequency {
	case Monthly:
		// Last complete month (previous month)
		year, month, _ := now.Date()
		if month == 1 {
			// If current month is january last complete month is december
			return time.Date(year-1, 12, 1, 0, 0, 0, 0, time.UTC), nil
		}
		return time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC), nil

	case Quarterly:
		year := now.Year()
		currentMonth := now.Month()

		if currentMonth >= 10 {
			return time.Date(year, 7, 1, 0, 0, 0, 0, time.UTC), nil
		} else if currentMonth >= 7 {
			return time.Date(year, 4, 1, 0, 0, 0, 0, time.UTC), nil
		} else if currentMonth >= 4 {
			return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), nil
		} else {
			return time.Date(year-1, 10, 1, 0, 0, 0, 0, time.UTC), nil
		}
	default:
		// Return error if Monthly or Quarterly is not provided as BillingFrequency
		return time.Time{}, fmt.Errorf("invalid billing frequency: %s", frequency)
	}

}

// Creates list of available periods
func GenerateAvailablePeriods(consumerStartDate time.Time, frequency BillingFrequency) ([]Period, error) {
	var periods []Period
	var current time.Time

	if frequency == Monthly {
		current = GetFirstCompleteMonth(consumerStartDate)
	} else if frequency == Quarterly {
		current = GetFirstCompleteQuarter(consumerStartDate)
	} else {
		return nil, fmt.Errorf("error in reading frequency: %s", frequency)
	}

	lastPeriod, err := GetLastCompletePeriod(frequency)
	if err != nil {
		log.Fatal("Error getting last period:", err)
	}

	for !current.After(lastPeriod) {
		var end time.Time
		var label string

		if frequency == Monthly {
			end = current.AddDate(0, 1, -1)
			label = current.Format("January 2006")
		} else {
			end = current.AddDate(0, 3, -1)
			quarter := ((int(current.Month()) - 1) / 3) + 1
			label = fmt.Sprintf("Q%d %d", quarter, current.Year())
		}

		periods = append(periods, Period{
			Start:     current,
			End:       end,
			Label:     label,
			Frequency: frequency,
		})

		if frequency == Monthly {
			current = current.AddDate(0, 1, 0)
		} else {
			current = current.AddDate(0, 3, 0)
		}
	}

	return periods, nil
}

// Lets user choose from available periods
func SelectPeriod(periods []Period) Period {
	options := make([]string, len(periods))
	for i, period := range periods {
		options[i] = period.Label
	}

	choice := utils.GetSimpleChoice("Available periods for calculation", options)
	return periods[choice]
}

// Shows the selected period details
func DisplaySelectedPeriod(period Period) {
	utils.PrintSuccess(fmt.Sprintf("Selected period: %s", period.Label))
	utils.PrintInfo(fmt.Sprintf("From: %s", period.Start.Format("2006-01-02")))
	utils.PrintInfo(fmt.Sprintf("To: %s", period.End.Format("2006-01-02")))
}
