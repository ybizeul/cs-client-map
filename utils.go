package main

import (
	"fmt"
	"os"
	"time"
)

func yesterdayMidnightUnix() int64 {
	t := time.Now()
	y := t.Add(-time.Hour * 24)
	m := time.Date(y.Year(), y.Month(), y.Day(), 0, 0, 0, 0, time.Local)
	return m.UnixMilli()
}
func todayMidnightUnix() int64 {
	t := time.Now()
	m := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	return m.UnixMilli()
}

func printStdErr(f string, args ...any) {
	fmt.Fprintf(os.Stderr, f, args...)
}
