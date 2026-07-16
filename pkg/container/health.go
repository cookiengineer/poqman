package container

import "time"

type HealthConfig struct {
	Test        []string      `json:"test"`
	Interval    time.Duration `json:"interval"`
	Timeout     time.Duration `json:"timeout"`
	Retries     int           `json:"retries"`
	StartPeriod time.Duration `json:"startPeriod"`
}

type HealthStatus string

const (
	HealthStarting   HealthStatus = "starting"
	HealthHealthy    HealthStatus = "healthy"
	HealthUnhealthy  HealthStatus = "unhealthy"
)

type HealthState struct {
	Status      HealthStatus `json:"status"`
	FailingStreak int         `json:"failingStreak"`
	LastCheck   time.Time    `json:"lastCheck"`
}
