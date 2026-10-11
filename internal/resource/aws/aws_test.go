package aws

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/awsid"
	"github.com/tomatool/tomato/internal/config"
)

const testRole = "arn:aws:iam::123456789012:role/path/my-service"

func newTestAWS(t *testing.T, options map[string]any) *AWS {
	t.Helper()
	a, err := New("aws", config.Resource{Type: "aws", Options: options}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = a.Cleanup(context.Background()) })
	return a
}

func stsCall(t *testing.T, a *AWS, form url.Values, header http.Header) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, a.URL()+"/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("calling STS: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestAWS_Defaults(t *testing.T) {
	a, _ := New("aws", config.Resource{Type: "aws"}, nil)
	if a.region != "us-east-1" || a.roleARN != awsid.DefaultRoleARN || a.sessionName != "tomato" || a.ambient {
		t.Errorf("defaults: %+v", a)
	}
}

func TestAWS_InitIsIdempotent(t *testing.T) {
	a := newTestAWS(t, nil)
	url, dir := a.URL(), a.dir
	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("second Init: %v", err)
	}
	if a.URL() != url || a.dir != dir {
		t.Errorf("second Init restarted the resource: %s %s -> %s %s", url, dir, a.URL(), a.dir)
	}
}

func TestAWS_AppEnvIsIRSA(t *testing.T) {
	a := newTestAWS(t, map[string]any{"region": "eu-central-1", "role_arn": testRole, "session_name": "svc"})
	env := a.AppEnv()

	want := map[string]string{
		"AWS_REGION":                "eu-central-1",
		"AWS_ROLE_ARN":              testRole,
		"AWS_ROLE_SESSION_NAME":     "svc",
		"AWS_ENDPOINT_URL_STS":      a.URL(),
		"AWS_EC2_METADATA_DISABLED": "true",
	}
	for k, v := range want {
		if env.Set[k] != v {
			t.Errorf("%s = %q, want %q", k, env.Set[k], v)
		}
	}
	for _, k := range []string{"AWS_WEB_IDENTITY_TOKEN_FILE", "AWS_SHARED_CREDENTIALS_FILE", "AWS_CONFIG_FILE"} {
		if _, err := os.Stat(env.Set[k]); err != nil {
			t.Errorf("%s points at %q: %v", k, env.Set[k], err)
		}
	}
	unset := strings.Join(env.Unset, " ")
	for _, k := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_PROFILE"} {
		if !strings.Contains(unset, k) {
			t.Errorf("%s is not removed from the inherited environment", k)
		}
	}
}

func TestAWS_CredentialsFile(t *testing.T) {
	without := newTestAWS(t, nil)
	data, _ := os.ReadFile(without.AppEnv().Set["AWS_SHARED_CREDENTIALS_FILE"])
	if strings.Contains(string(data), "aws_access_key_id") {
		t.Errorf("credentials file has an identity without ambient_identity:\n%s", data)
	}

	with := newTestAWS(t, map[string]any{"ambient_identity": true})
	data, _ = os.ReadFile(with.AppEnv().Set["AWS_SHARED_CREDENTIALS_FILE"])
	if !strings.Contains(string(data), "[default]") || !strings.Contains(string(data), awsid.Ambient("aws").AccessKeyID) {
		t.Errorf("credentials file lacks the ambient identity:\n%s", data)
	}
}

func TestAWS_AssumeRoleWithWebIdentity(t *testing.T) {
	a := newTestAWS(t, nil)
	status, body := stsCall(t, a, url.Values{
		"Action":           {"AssumeRoleWithWebIdentity"},
		"RoleArn":          {testRole},
		"RoleSessionName":  {"svc"},
		"WebIdentityToken": {"token"},
	}, nil)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var resp stsAssumeRoleWithWebIdentityResponse
	if err := xml.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("response is not STS XML: %v\n%s", err, body)
	}
	creds := resp.Result.Credentials
	if creds.AccessKeyID != awsid.Session(testRole).AccessKeyID || creds.SessionToken == "" || creds.Expiration == "" {
		t.Errorf("credentials %+v", creds)
	}
	if got := resp.Result.AssumedRoleUser.Arn; got != "arn:aws:sts::123456789012:assumed-role/my-service/svc" {
		t.Errorf("assumed role ARN %q", got)
	}
	if err := a.roleWasAssumed(testRole); err != nil {
		t.Error(err)
	}
	if err := a.roleWasNotAssumed(testRole); err == nil {
		t.Error("roleWasNotAssumed passed for an assumed role")
	}
}

func TestAWS_AssumeRole(t *testing.T) {
	a := newTestAWS(t, nil)
	status, body := stsCall(t, a, url.Values{"Action": {"AssumeRole"}, "RoleArn": {testRole}, "RoleSessionName": {"svc"}}, nil)
	if status != http.StatusOK || !strings.Contains(body, "<AssumeRoleResponse") {
		t.Fatalf("status %d: %s", status, body)
	}
	if err := a.roleWasAssumed(testRole); err != nil {
		t.Error(err)
	}
}

func TestAWS_STSErrors(t *testing.T) {
	a := newTestAWS(t, nil)
	for name, tc := range map[string]struct {
		form url.Values
		code string
	}{
		"missing role":   {url.Values{"Action": {"AssumeRole"}}, "MissingParameter"},
		"missing token":  {url.Values{"Action": {"AssumeRoleWithWebIdentity"}, "RoleArn": {testRole}}, "MissingParameter"},
		"unknown action": {url.Values{"Action": {"DecodeAuthorizationMessage"}}, "InvalidAction"},
	} {
		status, body := stsCall(t, a, tc.form, nil)
		if status != http.StatusBadRequest || !strings.Contains(body, "<Code>"+tc.code+"</Code>") {
			t.Errorf("%s: status %d, body %s", name, status, body)
		}
	}
	if err := a.roleWasNotAssumed(testRole); err != nil {
		t.Errorf("a failed call was journaled: %v", err)
	}
	if err := a.roleWasAssumed(testRole); err == nil || !strings.Contains(err.Error(), "never assumed") {
		t.Errorf("roleWasAssumed: %v", err)
	}
}

func TestAWS_GetCallerIdentity(t *testing.T) {
	a := newTestAWS(t, map[string]any{"role_arn": testRole, "ambient_identity": true})
	for key, wantArn := range map[string]string{
		awsid.Session(testRole).AccessKeyID: "arn:aws:sts::123456789012:assumed-role/my-service/tomato",
		awsid.Ambient("aws").AccessKeyID:    "arn:aws:iam::123456789012:user/tomato-ambient",
		"SOMEONEELSE000000000":              "arn:aws:iam::123456789012:user/unknown",
	} {
		header := http.Header{"Authorization": {"AWS4-HMAC-SHA256 Credential=" + key + "/20260929/us-east-1/sts/aws4_request, SignedHeaders=host, Signature=x"}}
		status, body := stsCall(t, a, url.Values{"Action": {"GetCallerIdentity"}}, header)
		if status != http.StatusOK || !strings.Contains(body, "<Arn>"+wantArn+"</Arn>") {
			t.Errorf("%s: status %d, body %s", key, status, body)
		}
	}
}

func TestAWS_ResetKeepsTheJournal(t *testing.T) {
	a := newTestAWS(t, nil)
	stsCall(t, a, url.Values{"Action": {"AssumeRole"}, "RoleArn": {testRole}}, nil)
	if err := a.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.roleWasAssumed(testRole); err != nil {
		t.Errorf("Reset dropped an assumption made at startup: %v", err)
	}
}

func TestAWS_StepsAndCleanup(t *testing.T) {
	a := newTestAWS(t, nil)
	if n := len(a.Steps().Steps); n != 2 {
		t.Errorf("%d steps", n)
	}
	dir := a.dir
	if err := a.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Cleanup left %s behind", dir)
	}
}

func TestSplitRoleARN(t *testing.T) {
	for arn, want := range map[string][2]string{
		"arn:aws:iam::123456789012:role/svc":         {"123456789012", "svc"},
		"arn:aws:iam::123456789012:role/path/to/svc": {"123456789012", "svc"},
		"not-an-arn": {"000000000000", "not-an-arn"},
	} {
		account, name := splitRoleARN(arn)
		if account != want[0] || name != want[1] {
			t.Errorf("%s: %s %s, want %s %s", arn, account, name, want[0], want[1])
		}
	}
}
