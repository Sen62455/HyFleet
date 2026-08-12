package nodeops

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

type fileOwnership struct {
	uid uint64
	gid uint64
}

func ownershipOf(info os.FileInfo) (fileOwnership, bool) {
	if info == nil {
		return fileOwnership{}, false
	}
	stat := reflect.ValueOf(info.Sys())
	if stat.Kind() == reflect.Pointer && !stat.IsNil() {
		stat = stat.Elem()
	}
	if stat.Kind() != reflect.Struct {
		return fileOwnership{}, false
	}
	uid := stat.FieldByName("Uid")
	gid := stat.FieldByName("Gid")
	if !uid.IsValid() || !gid.IsValid() || !uid.CanUint() || !gid.CanUint() {
		return fileOwnership{}, false
	}
	return fileOwnership{uid: uid.Uint(), gid: gid.Uint()}, true
}

func fileOwnedByRoot(info os.FileInfo) bool {
	owner, ok := ownershipOf(info)
	return ok && owner.uid == 0 && owner.gid == 0
}

func fileOwnedByRootUser(info os.FileInfo) bool {
	owner, ok := ownershipOf(info)
	return ok && owner.uid == 0
}

func sameFileOwnership(left, right os.FileInfo) bool {
	if runtime.GOOS == "windows" {
		return true
	}
	leftOwner, leftOK := ownershipOf(left)
	rightOwner, rightOK := ownershipOf(right)
	return leftOK && rightOK && leftOwner == rightOwner
}

func exactPermissions(mode, expected os.FileMode) bool {
	return runtime.GOOS == "windows" || mode.Perm() == expected
}

func samePermissions(left, right os.FileMode) bool {
	return runtime.GOOS == "windows" || left.Perm() == right.Perm()
}

func pathWithin(path, root string) bool {
	cleanPath := filepath.Clean(path)
	cleanRoot := filepath.Clean(root)
	if cleanPath == cleanRoot {
		return true
	}
	relative, err := filepath.Rel(cleanRoot, cleanPath)
	return err == nil && relative != "." && relative != ".." &&
		!filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func requiresRealityConfigRootOwner(path string) bool {
	return runtime.GOOS != "windows" && pathWithin(path, "/etc/sing-box")
}

func requiresBackupRootOwner(path string) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	return pathWithin(path, "/var/lib/hyfleet-backups") ||
		pathWithin(path, "/var/lib/hyfleet-backups-lab")
}
