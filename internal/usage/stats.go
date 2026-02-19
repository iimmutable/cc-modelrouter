package usage

// Summary represents aggregated usage summary.
type Summary struct {
	TotalRequests  int
	TotalTokens    int
	TotalFallbacks int
}

// RouteStats represents stats for a single route.
type RouteStats struct {
	Route     string
	Requests  int
	Tokens    int
	Fallbacks int
}

// ModelStats represents stats for a single model.
type ModelStats struct {
	Model    string
	Requests int
	Tokens   int
}

// AggregateSummary computes overall summary from records.
func AggregateSummary(records []*Record) Summary {
	var s Summary
	for _, r := range records {
		s.TotalRequests++
		s.TotalTokens += r.Tokens
		s.TotalFallbacks += r.Fallbacks
	}
	return s
}

// AggregateByRoute groups records by route.
func AggregateByRoute(records []*Record) map[string]*RouteStats {
	result := make(map[string]*RouteStats)
	for _, r := range records {
		stats, ok := result[r.Route]
		if !ok {
			stats = &RouteStats{Route: r.Route}
			result[r.Route] = stats
		}
		stats.Requests++
		stats.Tokens += r.Tokens
		stats.Fallbacks += r.Fallbacks
	}
	return result
}

// AggregateByModel groups records by model.
func AggregateByModel(records []*Record) map[string]*ModelStats {
	result := make(map[string]*ModelStats)
	for _, r := range records {
		stats, ok := result[r.Model]
		if !ok {
			stats = &ModelStats{Model: r.Model}
			result[r.Model] = stats
		}
		stats.Requests++
		stats.Tokens += r.Tokens
	}
	return result
}
