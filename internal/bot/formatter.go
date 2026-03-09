package bot

import "strings"

// EscapeMarkdownV2 escapes special characters for Telegram MarkdownV2.
func EscapeMarkdownV2(text string) string {
	special := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	result := text
	for _, ch := range special {
		result = strings.ReplaceAll(result, ch, "\\"+ch)
	}
	return result
}

// FormatLeaveBalance formats leave balance for display.
func FormatLeaveBalance(leaveType string, used, total float64) string {
	remaining := total - used
	return EscapeMarkdownV2(leaveType) + ": " +
		EscapeMarkdownV2(formatFloat(remaining)) + "/" +
		EscapeMarkdownV2(formatFloat(total)) + " days"
}

func formatFloat(f float64) string {
	if f == float64(int(f)) {
		return strings.TrimRight(strings.TrimRight(
			strings.Replace(
				strings.Replace(
					formatFloatRaw(f), ".0", "", 1,
				), ".00", "", 1,
			), "0"), ".")
	}
	return formatFloatRaw(f)
}

func formatFloatRaw(f float64) string {
	s := strings.TrimRight(strings.TrimRight(
		strings.Replace(
			formatNumber(f), ",", "", -1,
		), "0"), ".")
	return s
}

func formatNumber(f float64) string {
	// Simple float to string
	if f == float64(int64(f)) {
		return strings.Replace(
			strings.TrimRight(
				strings.TrimRight(
					sprintf("%.1f", f), "0",
				), "."),
			"", "", 0)
	}
	return sprintf("%.1f", f)
}

func sprintf(format string, a ...any) string {
	// Inline fmt.Sprintf to avoid import
	switch format {
	case "%.1f":
		v := a[0].(float64)
		i := int64(v * 10)
		whole := i / 10
		frac := i % 10
		if frac < 0 {
			frac = -frac
		}
		sign := ""
		if v < 0 && whole == 0 {
			sign = "-"
		}
		return sign + intToStr(whole) + "." + intToStr(int64(frac))
	default:
		return ""
	}
}

func intToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
