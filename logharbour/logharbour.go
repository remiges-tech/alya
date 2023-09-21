package logharbour

import (
	"fmt"
)

// New returns a new LogHarbour
func New() *LogHarbour {
	return &LogHarbour{}
}

type LogHarbour struct {
}

func (l *LogHarbour) Log(v ...interface{}) {
	fmt.Println(v...)
}
