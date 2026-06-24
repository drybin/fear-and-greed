package model

import (
	"fmt"
	"strconv"
	"time"

	"github.com/drybin/fear-and-greed/pkg/wrap"
)

type DayInfo struct {
	Date  time.Time
	Index int
	Price float64
}

func NewDayInfo(data []string) (*DayInfo, error) {
	layout := "2006-01-02 15:04:05"
	date, err := time.Parse(layout, data[0])
	if err != nil {
		fmt.Println(err)
		return nil, wrap.Errorf("failed to parse time in line from csv file: %w", err)
	}

	fear, err := strconv.Atoi(data[1])
	if err != nil {
		return nil, wrap.Errorf("failed to parse fear index in line from csv file: %w", err)
	}

	price, err := strconv.ParseFloat(data[2], 64) // 64 for float64 precision
	if err != nil {
		return nil, wrap.Errorf("failed to parse price in line from csv file: %w", err)
	}

	return &DayInfo{
		Date:  date,
		Index: fear,
		Price: price,
	}, nil
}
