package awsid

import (
	"strings"
	"testing"
)

func TestSession_IsStablePerRoleAndDistinctAcrossRoles(t *testing.T) {
	a := Session("arn:aws:iam::000000000000:role/a")
	if again := Session("arn:aws:iam::000000000000:role/a"); again != a {
		t.Errorf("Session is not stable: %+v then %+v", a, again)
	}
	if b := Session("arn:aws:iam::000000000000:role/b"); b.AccessKeyID == a.AccessKeyID {
		t.Errorf("two roles share access key id %s", a.AccessKeyID)
	}
	if a.SessionToken == "" {
		t.Error("a role session has no session token")
	}
}

func TestAccessKeyIDs_LookLikeAWSButNotLikeRealKeys(t *testing.T) {
	for _, creds := range []Credentials{Session(DefaultRoleARN), Ambient("aws")} {
		id := creds.AccessKeyID
		if len(id) != 20 {
			t.Errorf("%s: length %d, want 20", id, len(id))
		}
		if strings.HasPrefix(id, "AKIA") || strings.HasPrefix(id, "ASIA") {
			t.Errorf("%s: carries a real AWS key prefix that secret scanners flag", id)
		}
		if id != strings.ToUpper(id) {
			t.Errorf("%s: not upper case", id)
		}
	}
}

func TestAmbient_IsNeverARoleSession(t *testing.T) {
	ambient := Ambient("aws")
	if ambient.AccessKeyID == Session(DefaultRoleARN).AccessKeyID {
		t.Error("the ambient identity equals the default role session")
	}
	if ambient.SessionToken != "" {
		t.Error("the ambient identity has a session token; it is a long-term identity")
	}
	if Ambient("other").AccessKeyID == ambient.AccessKeyID {
		t.Error("two resources share an ambient identity")
	}
}
