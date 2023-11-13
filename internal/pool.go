package internal

import (
	"sync"
	"time"
)

var (
	tickerPool = sync.Pool{
		New: func() any {
			return time.NewTicker(10)
		},
	}
)
