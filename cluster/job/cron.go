package job

import (
	"time"

	"github.com/robfig/cron/v3"
)

var standardCronParser = cron.NewParser(
	cron.Minute |
		cron.Hour |
		cron.Dom |
		cron.Month |
		cron.Dow |
		cron.Descriptor,
)

// NextSchedule returns the next scheduled time and the next definition status.
func NextSchedule(definition *Definition, scheduledAt time.Time) (time.Time, DefinitionStatus, error) {
	if definition == nil {
		return time.Time{}, "", ErrInvalidDefinition
	}

	switch definition.ScheduleType {
	case ScheduleOnce, ScheduleDelay:
		return time.Time{}, DefinitionDisabled, nil
	case ScheduleFixed:
		duration, err := time.ParseDuration(definition.ScheduleExpr)
		if err != nil || duration <= 0 {
			return time.Time{}, "", ErrInvalidSchedule
		}

		return scheduledAt.Add(duration), DefinitionEnabled, nil
	case ScheduleCron:
		location := time.UTC
		if definition.Timezone != "" {
			var err error
			location, err = time.LoadLocation(definition.Timezone)
			if err != nil {
				return time.Time{}, "", ErrInvalidSchedule
			}
		}

		schedule, err := standardCronParser.Parse(definition.ScheduleExpr)
		if err != nil {
			return time.Time{}, "", ErrInvalidSchedule
		}

		next := schedule.Next(scheduledAt.In(location))
		return next.UTC(), DefinitionEnabled, nil
	default:
		return time.Time{}, "", ErrUnsupportedSchedule
	}
}
