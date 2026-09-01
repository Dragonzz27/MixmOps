package incident

import (
	"testing"
	"time"
)

func TestParseTimeoutAndTruncateText(t *testing.T) {
	duration, err := parseTimeout("2m", time.Minute, "test")
	if err != nil || duration != 2*time.Minute {
		t.Fatalf("duration=%s err=%v", duration, err)
	}
	if _, err := parseTimeout("invalid", time.Minute, "test"); err == nil {
		t.Fatal("expected invalid timeout error")
	}
	if got := truncateText("集群故障分析", 2); got != "集群\n...<context truncated>" {
		t.Fatalf("unexpected truncation %q", got)
	}
}
