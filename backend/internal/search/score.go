package search

import (
	"math"
	"time"
)

const (
	BudgetWeight      = 0.7
	RecencyWeight     = 0.3
	RecencyWindowDays = 90.0
)

// Score returns relevance in 0..100. Both terms are linear and clamped to 0..1:
// budget fit falls off with relative distance from the target (symmetric over
// and under), and recency falls to 0 at RecencyWindowDays.
func Score(l Listing, targetBudget *float64, now time.Time) float64 {
	ageDays := math.Max(0, math.Floor(now.Sub(l.ListedDate.Time).Hours()/24))
	recency := math.Max(0, 1-ageDays/RecencyWindowDays)
	if targetBudget == nil {
		return round1(100 * recency)
	}
	t := *targetBudget
	budgetFit := math.Max(0, 1-math.Abs(l.Price-t)/t)
	return round1(100 * (BudgetWeight*budgetFit + RecencyWeight*recency))
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }
