package service

import (
	"context"
	"strings"
	"unicode/utf8"
)

type CheckAvailabilityRepository interface {
	CheckNameAvailability(ctx context.Context, clientID, name string, excludeID *int64) (bool, error)
}

type CheckAvailabilityService struct {
	repository CheckAvailabilityRepository
}

func NewCheckAvailabilityService(repository CheckAvailabilityRepository) *CheckAvailabilityService {
	return &CheckAvailabilityService{repository: repository}
}

func (s *CheckAvailabilityService) CheckAvailability(ctx context.Context, clientID, name string, excludeID *int64) (bool, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > 120 || containsEmoji(trimmed) {
		return false, nil
	}
	return s.repository.CheckNameAvailability(ctx, clientID, trimmed, excludeID)
}

func containsEmoji(value string) bool {
	for _, r := range value {
		if isEmojiRune(r) {
			return true
		}
	}
	return false
}

func isEmojiRune(r rune) bool {
	return r == 0x00A9 || r == 0x00AE || r == 0x203C || r == 0x2049 || r == 0x2122 || r == 0x2139 ||
		r == 0x3030 || r == 0x303D || r == 0x3297 || r == 0x3299 ||
		(r >= 0x2300 && r <= 0x23FF) || (r >= 0x2600 && r <= 0x27BF) || (r >= 0x1F000 && r <= 0x1FAFF)
}
