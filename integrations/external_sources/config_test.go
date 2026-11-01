package externalsources

import (
	"strings"
	"testing"
)

func TestValidSources(t *testing.T) {
	raw := `{"sources":[
 {"id":"nas-hires","type":"webdav","endpoint":"https://nas.example.com:5006","root":"Music/HiRes","secret_env":"NAVIDROME_SOURCE_NAS_KEY","read_only":true,"enabled":false},
 {"id":"external-s3","type":"s3","endpoint":"https://s3.example.com","bucket":"music.archive","region":"us-east-1","root":"Music","secret_env":"NAVIDROME_SOURCE_S3_KEY","read_only":true,"enabled":false},
 {"id":"sftp-vault","type":"sftp","endpoint":"sftp://files.example.com:22","root":"Music","secret_env":"NAVIDROME_SOURCE_SFTP_KEY","host_key_sha256":"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","read_only":true,"enabled":false}
 ]}`
	c, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Sources) != 3 {
		t.Fatal("unexpected source count")
	}
}
func TestRejectUnsafeSource(t *testing.T) {
	base := `{"sources":[{"id":"nas-hires","type":"webdav","endpoint":"https://nas.example.com","root":"Music","secret_env":"NAVIDROME_SOURCE_NAS_KEY","read_only":true,"enabled":false}]}`
	cases := []string{
		strings.Replace(base, `"read_only":true`, `"read_only":false`, 1),
		strings.Replace(base, "https://nas.example.com", "http://nas.example.com", 1),
		strings.Replace(base, "https://nas.example.com", "https://user:private@nas.example.com", 1),
		strings.Replace(base, "https://nas.example.com", "https://nas.example.com/mount", 1),
		strings.Replace(base, "https://nas.example.com", "https://nas.example.com?token=123", 1),
		strings.Replace(base, `"root":"Music"`, `"root":"../Music"`, 1),
		strings.Replace(base, "NAVIDROME_SOURCE_NAS_KEY", "inline-secret", 1),
		strings.Replace(base, `"type":"webdav"`, `"type":"ftp"`, 1),
		strings.Replace(base, `"id":"nas-hires"`, `"id":"../nas"`, 1),
		strings.Replace(base, `"enabled":false`, `"enabled":false,"unknown":1`, 1),
		base + "{}",
	}
	for i, c := range cases {
		if _, err := Parse([]byte(c)); err == nil {
			t.Fatalf("unsafe source accepted: case %d", i)
		}
	}
}
func TestNoSecretInError(t *testing.T) {
	c := `{"sources":[{"id":"nas-hires","type":"webdav","endpoint":"https://user:SECRET_VALUE@nas.example.com","root":".","secret_env":"NAVIDROME_SOURCE_NAS_KEY","read_only":true,"enabled":false}]}`
	_, err := Parse([]byte(c))
	if err == nil || strings.Contains(err.Error(), "SECRET_VALUE") {
		t.Fatalf("invalid input accepted or leak: %v", err)
	}
}
func TestEmpty(t *testing.T) {
	if _, err := Parse([]byte(`{"sources":[]}`)); err != nil {
		t.Fatal(err)
	}
}
