package model

import "strings"

func IsRemotePath(p string) bool {
	for _, s := range []string{"bunny://", "s3://", "webdav://", "sftp://"} {
		if strings.HasPrefix(p, s) {
			return true
		}
	}
	return false
}
