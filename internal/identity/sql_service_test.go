package identity

import (
	"testing"
	"time"
)

func TestValidateLoadedSession(t *testing.T) {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		session Session
		active  bool
		wantErr error
	}{
		{
			name: "ok",
			session: Session{
				ID:        "session-1",
				ExpiresAt: now.Add(time.Hour),
			},
			active: true,
		},
		{
			name: "revoked",
			session: Session{
				ID:        "session-1",
				ExpiresAt: now.Add(time.Hour),
				RevokedAt: timePtr(now),
			},
			active:  true,
			wantErr: ErrSessionRevoked,
		},
		{
			name: "expired",
			session: Session{
				ID:        "session-1",
				ExpiresAt: now,
			},
			active:  true,
			wantErr: ErrSessionExpired,
		},
		{
			name: "inactive user",
			session: Session{
				ID:        "session-1",
				ExpiresAt: now.Add(time.Hour),
			},
			active:  false,
			wantErr: ErrInactiveUser,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLoadedSession(tc.session, tc.active, now)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if tc.wantErr != nil && err != tc.wantErr {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
