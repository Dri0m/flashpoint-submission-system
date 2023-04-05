package types

import "time"

const (
	DeviceFlowPending      = 0
	DeviceFlowErrorExpired = 1
	DeviceFlowErrorDenied  = 2
	DeviceFlowComplete     = 3
)

type DeviceFlowToken struct {
	DeviceCode      string            `json:"device_code"`
	UserCode        string            `json:"user_code"`
	VerificationURL string            `json:"verification_uri"`
	ExpiresIn       int64             `json:"expires_in"`
	Interval        int64             `json:"interval"`
	ExpiresAt       time.Time         `json:"-"`
	FlowState       int64             `json:"-"`
	AuthToken       map[string]string `json:"-"` // Explictly add this to responses when suitable
}

type DeviceFlowPollResponse struct {
	Error string `json:"error,omitempty"`
	Token string `json:"access_token,omitempty"`
}
