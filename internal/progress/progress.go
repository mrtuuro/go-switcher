package progress

import "fmt"

type Event struct {
	Stage   string
	Message string
	Current int64
	Total   int64
}

type Reporter func(Event)

func Emit(reporter Reporter, stage string, message string, current int64, total int64) {
	if reporter == nil {
		return
	}

	reporter(Event{
		Stage:   stage,
		Message: message,
		Current: current,
		Total:   total,
	})
}

func FormatBytes(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)

	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func FormatTransfer(current int64, total int64) string {
	if total <= 0 {
		return fmt.Sprintf("%s downloaded", FormatBytes(current))
	}

	percent := 0.0
	if total > 0 {
		percent = (float64(current) / float64(total)) * 100
		if percent > 100 {
			percent = 100
		}
	}

	return fmt.Sprintf("%s / %s (%.0f%%)", FormatBytes(current), FormatBytes(total), percent)
}
