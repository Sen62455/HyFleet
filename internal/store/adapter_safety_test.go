package store

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUpdateNodeRejectsAdapterChangeWithDependentState(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tests := []struct {
		name  string
		setup func(*testing.T, *Store, Node)
	}{
		{
			name: "assignment and credential",
			setup: func(t *testing.T, database *Store, node Node) {
				t.Helper()
				if _, _, err := database.CreateUser(t.Context(), NewUser{
					ID: uuid.NewString(), Username: "assigned-user", Enabled: true,
					NodeIDs: []string{node.ID}, Now: now,
				}, bytes.Repeat([]byte{0x71}, 32)); err != nil {
					t.Fatalf("CreateUser() error = %v", err)
				}
			},
		},
		{
			name: "credential without assignment",
			setup: func(t *testing.T, database *Store, node Node) {
				t.Helper()
				user, _, err := database.CreateUser(t.Context(), NewUser{
					ID: uuid.NewString(), Username: "unassigned-user", Enabled: true,
					NodeIDs: []string{node.ID}, Now: now,
				}, bytes.Repeat([]byte{0x72}, 32))
				if err != nil {
					t.Fatalf("CreateUser() error = %v", err)
				}
				if err := database.UnassignUser(t.Context(), user.ID, node.ID, now.Add(time.Second)); err != nil {
					t.Fatalf("UnassignUser() error = %v", err)
				}
				var assignments, credentials int
				if err := database.DB().QueryRowContext(t.Context(), `
					SELECT
						(SELECT COUNT(*) FROM node_user_assignments WHERE node_id = ?),
						(SELECT COUNT(*) FROM user_credentials WHERE node_id = ?)
				`, node.ID, node.ID).Scan(&assignments, &credentials); err != nil {
					t.Fatalf("read retained credential state: %v", err)
				}
				if assignments != 0 || credentials != 1 {
					t.Fatalf("retained state = assignments %d, credentials %d", assignments, credentials)
				}
			},
		},
		{
			name: "enrollment token",
			setup: func(t *testing.T, database *Store, node Node) {
				t.Helper()
				admin := Admin{
					ID: uuid.NewString(), Username: "adapter-admin", PasswordHash: "unused",
					CreatedAt: now, UpdatedAt: now,
				}
				if err := database.CreateAdmin(t.Context(), admin); err != nil {
					t.Fatalf("CreateAdmin() error = %v", err)
				}
				if _, err := database.CreateEnrollmentToken(
					t.Context(), node.ID, admin.ID, now, 10*time.Minute,
				); err != nil {
					t.Fatalf("CreateEnrollmentToken() error = %v", err)
				}
			},
		},
		{
			name: "agent binding",
			setup: func(t *testing.T, database *Store, node Node) {
				t.Helper()
				if _, err := database.DB().ExecContext(t.Context(), `
					UPDATE nodes SET agent_installation_id = ? WHERE id = ?
				`, uuid.NewString(), node.ID); err != nil {
					t.Fatalf("bind Agent: %v", err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database, err := Open(t.Context(), filepath.Join(t.TempDir(), "server.db"))
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			defer database.Close()
			node := createTestNode(t, database, "adapter-node", "native_hysteria2", now)
			test.setup(t, database, node)
			before, err := database.GetNode(t.Context(), node.ID)
			if err != nil {
				t.Fatalf("GetNode(before) error = %v", err)
			}

			_, err = database.UpdateNode(t.Context(), node.ID, adapterUpdate(before, "s_ui", now.Add(2*time.Second)))
			if !errors.Is(err, ErrConflict) {
				t.Fatalf("UpdateNode(adapter change) error = %v, want ErrConflict", err)
			}
			after, err := database.GetNode(t.Context(), node.ID)
			if err != nil {
				t.Fatalf("GetNode(after) error = %v", err)
			}
			if after.AdapterType != before.AdapterType || after.DesiredVersion != before.DesiredVersion {
				t.Fatalf("rejected adapter change mutated node: before=%#v after=%#v", before, after)
			}
		})
	}
}

func TestUpdateNodeAllowsAdapterChangeWithoutDependentState(t *testing.T) {
	database, err := Open(t.Context(), filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	now := time.Now().UTC().Truncate(time.Millisecond)
	node := createTestNode(t, database, "pristine-adapter-node", "native_hysteria2", now)

	updated, err := database.UpdateNode(
		t.Context(), node.ID, adapterUpdate(node, "s_ui", now.Add(time.Second)),
	)
	if err != nil {
		t.Fatalf("UpdateNode(pristine adapter change) error = %v", err)
	}
	if updated.AdapterType != "s_ui" || updated.DesiredVersion != node.DesiredVersion+1 {
		t.Fatalf("updated node = %#v", updated)
	}
}

func TestRequestUserKickAtomicallyQueuesMixedManagedTargets(t *testing.T) {
	database, err := Open(t.Context(), filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	now := time.Now().UTC().Truncate(time.Millisecond)
	native := createTestNode(t, database, "mixed-native", "native_hysteria2", now)
	reality := createVLESSRealityTestNode(t, database, "mixed-reality", now)
	user, _, err := database.CreateUser(t.Context(), NewUser{
		ID: uuid.NewString(), Username: "mixed-kick-user", Enabled: true,
		NodeIDs: []string{native.ID, reality.ID}, Now: now,
	}, bytes.Repeat([]byte{0x73}, 32))
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	nativeBefore, err := database.GetNode(t.Context(), native.ID)
	if err != nil {
		t.Fatalf("GetNode(native before) error = %v", err)
	}

	count, err := database.RequestUserKick(t.Context(), user.ID, "", now.Add(time.Second))
	if count != 2 || err != nil {
		t.Fatalf("RequestUserKick(mixed global) = %d, error = %v", count, err)
	}
	var queued int
	if err := database.DB().QueryRowContext(t.Context(), `
		SELECT COUNT(*) FROM node_kick_targets WHERE user_id = ?
	`, user.ID).Scan(&queued); err != nil {
		t.Fatalf("count queued kicks: %v", err)
	}
	nativeAfter, err := database.GetNode(t.Context(), native.ID)
	if err != nil {
		t.Fatalf("GetNode(native after) error = %v", err)
	}
	if queued != 2 || nativeAfter.DesiredVersion != nativeBefore.DesiredVersion+1 {
		t.Fatalf("mixed global kick state: queued=%d before=%d after=%d",
			queued, nativeBefore.DesiredVersion, nativeAfter.DesiredVersion)
	}

	count, err = database.RequestUserKick(t.Context(), user.ID, native.ID, now.Add(2*time.Second))
	if count != 1 || err != nil {
		t.Fatalf("RequestUserKick(targeted native) = %d, error = %v", count, err)
	}
}

func adapterUpdate(node Node, adapter string, now time.Time) UpdateNode {
	return UpdateNode{
		Name: node.Name, Provider: node.Provider, Region: node.Region,
		AdapterType: adapter, PublicHost: node.PublicHost, PublicPort: node.PublicPort,
		SNI: node.SNI, TLSInsecure: node.TLSInsecure,
		TLSCertFingerprint: node.TLSCertFingerprint,
		TLSPublicKeySHA256: node.TLSPublicKeySHA256,
		Enabled:            node.Enabled,
		Now:                now,
	}
}
