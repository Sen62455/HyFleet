package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const realityBuildVersion = "1.13.18-hyfleet-utls1.8.7"

func TestSingBoxRealitySupplyChainContract(t *testing.T) {
	repositoryRoot := filepath.Clean("..")
	read := func(path string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		return string(content)
	}

	buildScript := read("scripts/build-sing-box-reality.sh")
	installer := read("deploy/install-agent.sh")
	helper := read("internal/nodeops/reality.go")
	releaseBuilder := read("scripts/build-release.ps1")
	service := read("deploy/systemd/hyfleet-sing-box-reality.service")

	for _, required := range []string{
		`sing_box_commit="45ca32dcb966f07f97fc888fe8586e359dbe8405"`,
		`go_toolchain="go1.26.5"`,
		`utls_version="v1.8.7"`,
		`-buildvcs=false`,
		`-tags with_utls`,
		`-p 1`,
		`CGO_ENABLED=0`,
		`GOOS=linux`,
		`GOMODCACHE=`,
		`GOCACHE=`,
		`go mod verify`,
	} {
		if !strings.Contains(buildScript, required) {
			t.Errorf("build script is missing %q", required)
		}
	}
	if strings.Contains(buildScript, "git clone --branch") || strings.Contains(buildScript, "git checkout v1.13.18") {
		t.Fatal("build script trusts a movable tag instead of the pinned commit")
	}
	for name, content := range map[string]string{
		"build script": buildScript,
		"installer":    installer,
		"helper":       helper,
	} {
		if !strings.Contains(content, realityBuildVersion) {
			t.Errorf("%s does not pin %s", name, realityBuildVersion)
		}
	}
	if !strings.Contains(installer, "sha256sum") || !strings.Contains(installer, "source_reality_checksums") {
		t.Fatal("installer does not validate the bundled Reality checksum manifest")
	}
	if !strings.Contains(installer, `-perm /022`) {
		t.Fatal("installer does not reject a group- or world-writable Reality binary")
	}
	if !strings.Contains(installer, "refusing to adopt existing unmanaged Reality configuration") ||
		!strings.Contains(installer, `"${reality_identity}" "${reality_applied}"`) {
		t.Fatal("installer does not reject an existing Reality config without managed local state")
	}
	if strings.Contains(installer, `chown root:hyfleet-singbox "${reality_core_config}"`) ||
		strings.Contains(installer, `chmod 0640 "${reality_core_config}"`) {
		t.Fatal("installer silently normalizes and adopts an existing Reality configuration")
	}
	for _, required := range []string{
		`if mkdir -m 0750 /etc/sing-box 2>/dev/null; then`,
		`reality_config_dir_identity="$(stat -c '%d:%i' /etc/sing-box)"`,
		`"$(stat -c '%u:%g' /etc/sing-box)" == "0:${reality_group_id}"`,
		`find /etc/sing-box -maxdepth 0 -perm /022`,
		`runuser -u hyfleet-singbox -- test -x /etc/sing-box`,
	} {
		if !strings.Contains(installer, required) {
			t.Errorf("installer does not preserve an existing shared config directory; missing %q", required)
		}
	}
	if strings.Contains(installer, "install -d -o root -g hyfleet-singbox -m 0750 /etc/sing-box") {
		t.Fatal("installer must use atomic mkdir to distinguish a new config directory")
	}
	if !strings.Contains(releaseBuilder, `"sing-box-reality.sha256"`) {
		t.Fatal("release builder does not bundle the Reality checksum manifest")
	}
	if !strings.Contains(service, "ExecStart=/usr/bin/sing-box") ||
		!strings.Contains(service, "ExecStartPre=/usr/bin/test ! -L /usr/bin/sing-box") {
		t.Fatal("Reality service does not retain the fixed non-symlink binary contract")
	}
}

func TestSingBoxRealityManifest(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "deploy", "sing-box-reality.sha256"))
	if err != nil {
		t.Fatalf("ReadFile(manifest) error = %v", err)
	}
	linePattern := regexp.MustCompile(`^[0-9a-f]{64}  sing-box-` + regexp.QuoteMeta(realityBuildVersion) + `-linux-(amd64|arm64)$`)
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		line = strings.TrimSuffix(line, "\r")
		matches := linePattern.FindStringSubmatch(line)
		if matches == nil || seen[matches[1]] {
			t.Fatalf("invalid or duplicate manifest line: %q", line)
		}
		if strings.HasPrefix(line, strings.Repeat("0", 64)) {
			t.Fatalf("manifest retains a placeholder checksum: %q", line)
		}
		seen[matches[1]] = true
	}
	if !seen["amd64"] || !seen["arm64"] || len(seen) != 2 {
		t.Fatalf("manifest architectures = %v, want amd64 and arm64", seen)
	}
}
