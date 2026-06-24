package scanreport

import "time"

// AlgoCatalog describes an algorithm for the HTML report.
type AlgoCatalog struct {
	Algorithms map[string]AlgoInfo `json:"algorithms"`
}

// AlgoInfo is a human-readable algorithm card.
type AlgoInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Short       string   `json:"short"`
	Summary     string   `json:"summary"`
	Tags        []string `json:"tags,omitempty"`
	NeedsVolume bool     `json:"needs_volume,omitempty"`
	DocSection  string   `json:"doc_section,omitempty"`
}

// Result is one algo × symbol × period snapshot.
type Result struct {
	Algo        string     `json:"algo"`
	Symbol      string     `json:"symbol"`
	Period      string     `json:"period"`
	PeriodLabel string     `json:"period_label"`
	CandleFrom  string     `json:"candle_from"`
	CandleTo    string     `json:"candle_to"`
	CandleCount int        `json:"candle_count"`
	UpdatedAt   string     `json:"updated_at"`
	Best        Best         `json:"best"`
	Sweep       []SweepRow   `json:"sweep,omitempty"`
	Trades      []TradeRecord `json:"trades,omitempty"`
}

// Best is the chosen best run (after sweep if applicable).
type Best struct {
	ParamLabel   string  `json:"param_label"`
	ParamValue   int     `json:"param_value,omitempty"`
	ProfitPct    float64 `json:"profit_pct"`
	ProfitUSD    float64 `json:"profit_usd"`
	TradeCount   int     `json:"trade_count"`
	OpenPosition bool    `json:"open_position"`
	WaitHoursAvg float64 `json:"wait_hours_avg,omitempty"`
	SMAPeriod    int     `json:"sma_period,omitempty"`
	LongTarget   int     `json:"long_target,omitempty"`
	ShortTarget  int     `json:"short_target,omitempty"`
	Leverage     int     `json:"leverage,omitempty"`
	MarginUSD    int     `json:"margin_usd,omitempty"`
	Liquidations int     `json:"liquidations,omitempty"`
	Bankrupt     bool    `json:"bankrupt,omitempty"`
}

// SweepRow is one row of a parameter sweep table.
type SweepRow struct {
	ParamLabel  string  `json:"param_label"`
	ParamValue  int     `json:"param_value,omitempty"`
	ProfitPct   float64 `json:"profit_pct"`
	ProfitUSD   float64 `json:"profit_usd"`
	TradeCount  int     `json:"trade_count"`
	SMAPeriod   int     `json:"sma_period,omitempty"`
	LongTarget  int     `json:"long_target,omitempty"`
	ShortTarget int     `json:"short_target,omitempty"`
}

// Manifest tracks the latest scan run metadata.
type Manifest struct {
	LastRunAt  string                    `json:"last_run_at"`
	DataDir    string                    `json:"data_dir"`
	Options    map[string]interface{}    `json:"options,omitempty"`
	Algorithms map[string]AlgoRunSummary `json:"algorithms"`
}

// AlgoRunSummary is per-algo metadata in manifest.
type AlgoRunSummary struct {
	UpdatedAt string   `json:"updated_at"`
	Symbols   []string `json:"symbols"`
	Periods   []string `json:"periods"`
}

// CandleRange from first/last candle timestamps.
type CandleRange struct {
	From  time.Time
	To    time.Time
	Count int
}
