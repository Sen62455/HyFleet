package store

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTrafficCycleStartClampsShortMonths(t *testing.T) {
	tests := []struct {
		at   time.Time
		day  int
		want time.Time
	}{
		{time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC), 31, time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)},
		{time.Date(2026, 2, 27, 12, 0, 0, 0, time.UTC), 31, time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)},
		{time.Date(2028, 2, 29, 1, 0, 0, 0, time.UTC), 31, time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC)},
		{time.Date(2026, 8, 13, 1, 0, 0, 0, time.UTC), 10, time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)},
	}
	for _, test := range tests {
		if got := trafficCycleStart(test.at, test.day); !got.Equal(test.want) {
			t.Fatalf("trafficCycleStart(%s, %d) = %s, want %s", test.at, test.day, got, test.want)
		}
	}
}

func TestNodeTrafficBudgetIsIdempotentCycleBoundAndCalibrated(t *testing.T) {
	ctx := t.Context()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	node, err := database.CreateNode(ctx, NewNode{
		ID: uuid.NewString(), Name: "budget-node", AdapterType: "native_hysteria2",
		Enabled: true, TrafficLimitBytes: 1000, TrafficResetDay: 10, Now: now,
	})
	if err != nil {
		t.Fatalf("CreateNode() error = %v", err)
	}
	user, _, err := database.CreateUser(ctx, NewUser{
		ID: uuid.NewString(), Username: "budget-user", Enabled: true,
		NodeIDs: []string{node.ID}, Now: now,
	}, bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	installationID := uuid.NewString()
	epoch := uuid.NewString()
	identity := AgentIdentity{
		NodeID: node.ID, InstallationID: installationID,
		AdapterType: "native_hysteria2", Enabled: true,
	}
	current := trafficBatch(
		installationID, epoch, 1, time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC),
		user.ID, 300, 500,
	)
	if result, err := database.IngestTrafficBatch(ctx, identity, current, now); err != nil || result.Status != "accepted" {
		t.Fatalf("IngestTrafficBatch(current) = %#v, %v", result, err)
	}
	if result, err := database.IngestTrafficBatch(ctx, identity, current, now.Add(time.Minute)); err != nil || result.Status != "duplicate" {
		t.Fatalf("IngestTrafficBatch(duplicate) = %#v, %v", result, err)
	}
	node, err = database.GetNode(ctx, node.ID)
	if err != nil || node.TrafficCycleUploadBytes != 300 ||
		node.TrafficCycleDownloadBytes != 500 || EffectiveNodeTrafficUsed(node) != 800 {
		t.Fatalf("current node budget = %#v, %v", node, err)
	}

	node, err = database.CalibrateNodeTraffic(ctx, node.ID, 850, now.Add(time.Hour))
	if err != nil || EffectiveNodeTrafficUsed(node) != 850 {
		t.Fatalf("CalibrateNodeTraffic() = %#v, %v", node, err)
	}
	afterCalibration := trafficBatch(
		installationID, epoch, 2, time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		user.ID, 20, 30,
	)
	if result, err := database.IngestTrafficBatch(ctx, identity, afterCalibration, now.Add(2*time.Hour)); err != nil || result.Status != "accepted" {
		t.Fatalf("IngestTrafficBatch(after calibration) = %#v, %v", result, err)
	}
	node, _ = database.GetNode(ctx, node.ID)
	if EffectiveNodeTrafficUsed(node) != 900 {
		t.Fatalf("calibrated effective traffic = %d, want 900", EffectiveNodeTrafficUsed(node))
	}

	old := trafficBatch(
		installationID, epoch, 3, time.Date(2026, 8, 9, 23, 59, 0, 0, time.UTC),
		user.ID, 100, 100,
	)
	if result, err := database.IngestTrafficBatch(ctx, identity, old, now.Add(3*time.Hour)); err != nil || result.Status != "accepted" {
		t.Fatalf("IngestTrafficBatch(old) = %#v, %v", result, err)
	}
	node, _ = database.GetNode(ctx, node.ID)
	if EffectiveNodeTrafficUsed(node) != 900 || node.TrafficUploadBytes != 420 || node.TrafficDownloadBytes != 630 {
		t.Fatalf("late old-cycle batch polluted budget: %#v", node)
	}
}
