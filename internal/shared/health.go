package shared

import "time"

const (
	HealthUp   = "UP"
	HealthDown = "DOWN"
)

type HealthStatus struct {
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks"`
	Timestamp string            `json:"timestamp"`
}

func NewHealthStatus(checks map[string]string) HealthStatus {
	status := HealthUp
	for _, v := range checks {
		if v != HealthUp {
			status = HealthDown
			break
		}
	}
	return HealthStatus{
		Status:    status,
		Checks:    checks,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}
