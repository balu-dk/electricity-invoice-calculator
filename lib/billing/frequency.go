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

// Returns the first complete quarter after startDate in Copenhagen timezone
func GetFirstCompleteQuarter(startDate time.Time) time.Time {
	// Convert to Copenhagen timezone
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")
	localStart := startDate.In(copenhagen)

	year := localStart.Year()

	quarters := []time.Time{
		time.Date(year, 1, 1, 0, 0, 0, 0, copenhagen),
		time.Date(year, 4, 1, 0, 0, 0, 0, copenhagen),
		time.Date(year, 7, 1, 0, 0, 0, 0, copenhagen),
		time.Date(year, 10, 1, 0, 0, 0, 0, copenhagen),
	}

	for _, quarter := range quarters {
		if quarter.After(localStart) {
			return quarter
		}
	}

	// If not quarter this year, return next year. Though, next year will not be valid for billing.
	return time.Date(year+1, 1, 1, 0, 0, 0, 0, copenhagen)
}

// Returns the first complete month after startDate in Copenhagen timezone
func GetFirstCompleteMonth(startDate time.Time) time.Time {
	// Convert to Copenhagen timezone
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")
	localStart := startDate.In(copenhagen)

	year, month, _ := localStart.Date()
	nextMonth := month + 1
	nextYear := year

	if nextMonth > 12 {
		nextMonth = 1
		nextYear = year + 1
	}

	return time.Date(nextYear, nextMonth, 1, 0, 0, 0, 0, copenhagen)
}

// Returns the last complete period in Copenhagen timezone
func GetLastCompletePeriod(frequency BillingFrequency) (time.Time, error) {
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")
	now := time.Now().In(copenhagen)

	switch frequency {
	case Monthly:
		// Last complete month (previous month)
		year, month, _ := now.Date()
		if month == 1 {
			// If current month is january last complete month is december
			return time.Date(year-1, 12, 1, 0, 0, 0, 0, copenhagen), nil
		}
		return time.Date(year, month-1, 1, 0, 0, 0, 0, copenhagen), nil

	case Quarterly:
		year := now.Year()
		currentMonth := now.Month()

		if currentMonth >= 10 {
			return time.Date(year, 7, 1, 0, 0, 0, 0, copenhagen), nil
		} else if currentMonth >= 7 {
			return time.Date(year, 4, 1, 0, 0, 0, 0, copenhagen), nil
		} else if currentMonth >= 4 {
			return time.Date(year, 1, 1, 0, 0, 0, 0, copenhagen), nil
		} else {
			return time.Date(year-1, 10, 1, 0, 0, 0, 0, copenhagen), nil
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

	switch frequency {
	case Monthly:
		current = GetFirstCompleteMonth(consumerStartDate)
	case Quarterly:
		current = GetFirstCompleteQuarter(consumerStartDate)
	default:
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
			// End date should be first day of next month (exclusive)
			// This means: November billing = 01-11-YYYY 00:00 to 01-12-YYYY 00:00
			end = current.AddDate(0, 1, 0)
			label = current.Format("January 2006")
		} else {
			// End date should be first day of next quarter (exclusive)
			// This means: Q4 billing = 01-10-YYYY 00:00 to 01-01-YYYY+1 00:00
			end = current.AddDate(0, 3, 0)
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
	utils.PrintInfo(fmt.Sprintf("From: %s (inclusive)", period.Start.Format("2006-01-02 15:04:05 MST")))
	utils.PrintInfo(fmt.Sprintf("To: %s (exclusive)", period.End.Format("2006-01-02 15:04:05 MST")))
	utils.PrintInfo(fmt.Sprintf("Time zone: %s", period.Start.Location()))
}
