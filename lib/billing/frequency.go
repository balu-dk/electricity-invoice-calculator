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

type CalculationType string

const (
	Historical CalculationType = "historical"
	Aconto     CalculationType = "aconto"
)

type Period struct {
	Start           time.Time
	End             time.Time
	Label           string
	Frequency       BillingFrequency
	CalculationType CalculationType
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

// Asks user to choose calculation type
func GetCalculationType() CalculationType {
	options := []string{
		"Historical calculation (based on actual consumption)",
		"Aconto calcuation (estimate for upcoming period)",
	}
	choice := utils.GetSimpleChoice("What type of calculation?", options)

	if choice == 0 {
		return Historical
	}

	return Aconto
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

// Returns next periods for aconto calculation
func GetNextAcontoPeriods(frequency BillingFrequency, numberOfPeriods int) ([]time.Time, error) {
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")
	now := time.Now().In(copenhagen)

	var startPeriods []time.Time

	switch frequency {
	case Monthly:
		// Start with the current month if we're path the 15, otherwise start with next month
		year, month, _ := now.Date()

		var startMonth time.Month
		var startYear int

		startMonth = month
		startYear = year

		for i := 0; i < numberOfPeriods; i++ {
			periodStart := time.Date(startYear, startMonth, 1, 0, 0, 0, 0, copenhagen)
			startPeriods = append(startPeriods, periodStart)

			startMonth++
			if startMonth > 12 {
				startMonth = 1
				startYear++
			}
		}

	case Quarterly:
		year := now.Year()
		currentMonth := now.Month()

		var nextQuarter time.Month
		if currentMonth < 3 {
			nextQuarter = 4 // Q2
		} else if currentMonth < 6 {
			nextQuarter = 7 // Q3
		} else if currentMonth < 9 {
			nextQuarter = 10 // Q4
		} else {
			nextQuarter = 1 // Q1 next year
			year++
		}

		for i := 0; i < numberOfPeriods; i++ {
			periodStart := time.Date(year, nextQuarter, 1, 0, 0, 0, 0, copenhagen)
			startPeriods = append(startPeriods, periodStart)

			nextQuarter += 3
			if nextQuarter > 12 {
				nextQuarter = 1
				year++
			}
		}

	default:
		return nil, fmt.Errorf("invalid billing frequency: %s", frequency)
	}

	return startPeriods, nil
}

// Creates list of available periods
func GenerateAvailablePeriods(consumerStartDate time.Time, frequency BillingFrequency, calculationType CalculationType) ([]Period, error) {
	var periods []Period
	var current time.Time

	if calculationType == Historical {
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
				end = current.AddDate(0, 1, 0)
				label = current.Format("January 2006")
			} else if frequency == Quarterly {
				end = current.AddDate(0, 3, 0)
				quarter := ((int(current.Month()) - 1) / 3) + 1
				label = fmt.Sprintf("Q%d %d", quarter, current.Year())
			}

			periods = append(periods, Period{
				Start:           current,
				End:             end,
				Label:           label,
				Frequency:       frequency,
				CalculationType: Historical,
			})

			if frequency == Monthly {
				current = current.AddDate(0, 1, 0)
			} else if frequency == Quarterly {
				current = current.AddDate(0, 3, 0)
			}
		}
	} else {
		numberOfPeriods := 6

		startPeriods, err := GetNextAcontoPeriods(frequency, numberOfPeriods)
		if err != nil {
			return nil, fmt.Errorf("error generating aconto periods: %w", err)
		}

		for _, start := range startPeriods {
			var end time.Time
			var label string

			if frequency == Monthly {
				end = start.AddDate(0, 1, 0)
				label = fmt.Sprintf("%s (Aconto)", start.Format("January 2006"))
			} else if frequency == Quarterly {
				end = start.AddDate(0, 3, 0)
				quarter := ((int(start.Month()) - 1) / 3) + 1
				label = fmt.Sprintf("Q%d %d (Aconto)", quarter, start.Year())
			}

			periods = append(periods, Period{
				Start:           start,
				End:             end,
				Label:           label,
				Frequency:       frequency,
				CalculationType: Aconto,
			})
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
	utils.PrintInfo(fmt.Sprintf("Calculation type: %s", period.CalculationType))
	utils.PrintInfo(fmt.Sprintf("From: %s (inclusive)", period.Start.Format("2006-01-02 15:04:05 MST")))
	utils.PrintInfo(fmt.Sprintf("To: %s (exclusive)", period.End.Format("2006-01-02 15:04:05 MST")))
	utils.PrintInfo(fmt.Sprintf("Time zone: %s", period.Start.Location()))
}
