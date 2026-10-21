package ai

import "time"

// Durable images can be explicitly recalled after a break. Implicit task
// instructions have an independent lifetime, even in an otherwise active group.
const implicitContextTTL = 30 * time.Minute

func recentContextTime(at, now time.Time) bool {
	return !at.IsZero() && !at.After(now.Add(time.Minute)) && now.Sub(at) < implicitContextTTL
}

func (s *Session) rebuildRecentMessages(now time.Time) {
	s.Messages = nil
	for _, turn := range s.turns {
		if recentContextTime(turn.At, now) {
			s.Messages = append(s.Messages, turn.messages()...)
		}
	}
}

func (s *Session) prepareRecentContext(now time.Time) {
	if len(s.turns) > 0 {
		s.rebuildRecentMessages(now)
	}
	if !s.lastTurnAt.IsZero() && !recentContextTime(s.lastTurnAt, now) {
		s.historySummary = ""
		s.SummaryTask = nil
		if len(s.turns) == 0 {
			s.Messages = nil
		}
	}
}
