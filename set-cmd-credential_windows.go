package main

import (
	"errors"
	"os/exec"
	"strconv"
)

func setCmdCredential(cmd *exec.Cmd, uid, gid int64) error {
	return errors.New("User and group cannot be set in Windows")
}

func parseUid(user string) (uint32, error) {
	n, err := strconv.Atoi(user)
	if err != nil && n > 0 {
		return uint32(n), nil
	}

	return uint32(0), errors.New("Cannot parse uid in Windows")
}

func parseGid(group string) (uint32, error) {
	n, err := strconv.Atoi(group)
	if err != nil && n > 0 {
		return uint32(n), nil
	}

	return uint32(0), errors.New("Cannot parse gid in Windows")
}
