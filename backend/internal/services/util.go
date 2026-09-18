package services

import "time"

func nowPtr() *time.Time {
	t := time.Now()
	return &t
}
