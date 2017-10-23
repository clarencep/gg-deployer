package main

//#include "id_linux.h"
import "C"

import (
	"errors"
	"os/exec"
	"strconv"
	"syscall"
)

func setCmdCredential(cmd *exec.Cmd, uid, gid int64) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}

	if cmd.SysProcAttr.Credential == nil {
		cmd.SysProcAttr.Credential = new(syscall.Credential)
		credential := cmd.SysProcAttr.Credential
		credential.Uid = uint32(syscall.Getuid())
		credential.Gid = uint32(syscall.Getgid())
		// credential.NoSetGroups = true
	}

	credential := cmd.SysProcAttr.Credential

	if uid >= 0 {
		credential.Uid = uint32(uid)
	}

	if gid >= 0 {
		credential.Gid = uint32(gid)
	}

	return nil
}

func parseUid(user string) (uint32, error) {
	n, err := strconv.Atoi(user)
	if err != nil && n > 0 {
		return uint32(n), nil
	}

	uid := C.id_user(C.CString(user))
	if uid < 0 {
		return 0, errors.New("Failed to get user id by `" + user + "`")
	}

	return uint32(uid), nil
}

func parseGid(group string) (uint32, error) {
	n, err := strconv.Atoi(group)
	if err != nil && n > 0 {
		return uint32(n), nil
	}

	gid := C.id_group(C.CString(group))
	if gid < 0 {
		return 0, errors.New("Failed to get group id by `" + group + "`")
	}

	return uint32(gid), nil
}
