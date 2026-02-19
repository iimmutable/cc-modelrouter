package usage

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
)

// FormatTokens formats a token count for display.
func FormatTokens(tokens int) string {
	switch {
	case tokens < 1_000:
		return fmt.Sprintf("%d", tokens)
	case tokens < 1_000_000:
		return fmt.Sprintf("%d,%03d", tokens/1000, tokens%1000)
	default:
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	}
}

// FormatUsage writes formatted usage statistics to the writer.
func FormatUsage(w io.Writer, instanceID, period string, records []*Record) {
	summary := AggregateSummary(records)
	byRoute := AggregateByRoute(records)
	byModel := AggregateByModel(records)

	// Header
	var title string
	if instanceID == "" {
		title = fmt.Sprintf("Usage Summary (%s, all instances)", period)
	} else {
		title = fmt.Sprintf("Usage Summary (%s, instance %s)", period, instanceID)
	}

	fmt.Fprintf(w, "%s\n", title)
	fmt.Fprintf(w, "  Requests: %s  |  Tokens: %s  |  Fallbacks: %d\n\n",
		formatNumber(summary.TotalRequests),
		FormatTokens(summary.TotalTokens),
		summary.TotalFallbacks)

	// By Route
	fmt.Fprintln(w, "By Route:")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  Route\tRequests\tTokens\tFallbacks")
	fmt.Fprintln(tw, "  ─────────────────────────────────────────────────")

	// Sort routes by name
	routes := make([]*RouteStats, 0, len(byRoute))
	for _, stats := range byRoute {
		routes = append(routes, stats)
	}
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Route < routes[j].Route
	})

	for _, stats := range routes {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%d\n",
			stats.Route,
			formatNumber(stats.Requests),
			FormatTokens(stats.Tokens),
			stats.Fallbacks)
	}
	tw.Flush()

	// By Model
	fmt.Fprintln(w, "\nBy Model:")
	tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  Model\tRequests\tTokens")
	fmt.Fprintln(tw, "  ──────────────────────────────────────────")

	// Sort models by token count descending
	models := make([]*ModelStats, 0, len(byModel))
	for _, stats := range byModel {
		models = append(models, stats)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Tokens > models[j].Tokens
	})

	for _, stats := range models {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n",
			stats.Model,
			formatNumber(stats.Requests),
			FormatTokens(stats.Tokens))
	}
	tw.Flush()
}

// formatNumber adds thousands separator.
func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result []string
	for i := len(s); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		result = append([]string{s[start:i]}, result...)
	}
	return strings.Join(result, ",")
}
