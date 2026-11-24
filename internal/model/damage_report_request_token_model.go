package model

import (
	"encoding/json"
	"time"
)

// DamageReportRequestToken is the Report Damaged idempotency record keyed by actor and token.
type DamageReportRequestToken struct {
	Token       string          `json:"token" column:"damage_report_request_tokens.token"`
	ActorID     int64           `json:"actor_id,string" column:"damage_report_request_tokens.actor_id"`
	RequestHash string          `json:"request_hash" column:"damage_report_request_tokens.request_hash"`
	Response    json.RawMessage `json:"response" column:"damage_report_request_tokens.response"`
	CreatedAt   time.Time       `json:"created_at" column:"damage_report_request_tokens.created_at"`
}
