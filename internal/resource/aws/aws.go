// Package aws provides tomato's `aws` resource: the IRSA-style identity tomato hands the
// application under test, and assertions on the roles it assumed at tomato's
// in-process STS.
package aws

import (
	"context"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	"github.com/tomatool/tomato/internal/awsid"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/resource"
)

// AWS emulates the identity a service is given on AWS through IRSA (IAM roles
// for service accounts): a web identity token and a role, which the SDK
// exchanges at STS for session credentials. tomato serves that STS itself and
// points the application at it, so the service under test gets its credentials
// the way it does in production. When that path breaks (the SDK cannot build its
// STS client, say) the credential chain falls through to whatever else it finds,
// and with ambient_identity that is an identity only a role session is told
// apart from: a kafka preset with auth: aws_msk_iam lets role sessions in and
// nothing else, as MSK does for a role with the kafka-cluster policy.
//
// The resource starts before the app (it implements resource.AppEnvProvider). Its
// journal of assumed roles covers the whole run and is not reset between
// scenarios, since services assume their role when they start.
type AWS struct {
	name        string
	config      config.Resource
	region      string
	roleARN     string
	sessionName string
	ambient     bool
	port        int

	mu          sync.Mutex
	initialized bool
	listener    net.Listener
	server      *http.Server
	dir         string
	assumed     []awsAssumption
}

type awsAssumption struct {
	Action  string
	RoleARN string
	Session string
	At      time.Time
}

// New creates an aws resource. Options: region (us-east-1), role_arn (the
// IRSA role, arn:aws:iam::000000000000:role/tomato), session_name (tomato),
// ambient_identity (false) and port (random).
func New(name string, cfg config.Resource, cm *container.Manager) (*AWS, error) {
	a := &AWS{
		name:        name,
		config:      cfg,
		region:      "us-east-1",
		roleARN:     awsid.DefaultRoleARN,
		sessionName: "tomato",
	}
	if v, ok := cfg.Options["region"].(string); ok && v != "" {
		a.region = v
	}
	if v, ok := cfg.Options["role_arn"].(string); ok && v != "" {
		a.roleARN = v
	}
	if v, ok := cfg.Options["session_name"].(string); ok && v != "" {
		a.sessionName = v
	}
	if v, ok := cfg.Options["ambient_identity"].(bool); ok {
		a.ambient = v
	}
	if v, ok := cfg.Options["port"].(int); ok {
		a.port = v
	}
	return a, nil
}

func (a *AWS) Name() string { return a.name }

// Init starts the STS endpoint and writes the files the app's SDK reads. It
// runs twice (before the app starts, and with every other resource), so the
// second call does nothing.
func (a *AWS) Init(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.initialized {
		return nil
	}

	dir, err := os.MkdirTemp("", "tomato-aws-"+a.name+"-")
	if err != nil {
		return fmt.Errorf("creating the credential directory: %w", err)
	}
	files := map[string]string{
		"web-identity-token": "tomato-web-identity-token-" + a.name + "\n",
		"credentials":        a.credentialsFile(),
		"config":             "[default]\nregion = " + a.region + "\n",
	}
	for file, content := range files {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o600); err != nil {
			return fmt.Errorf("writing %s: %w", file, err)
		}
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", a.port))
	if err != nil {
		return fmt.Errorf("starting the STS endpoint: %w", err)
	}

	a.dir = dir
	a.listener = listener
	a.port = listener.Addr().(*net.TCPAddr).Port
	a.server = &http.Server{Handler: http.HandlerFunc(a.serveSTS), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = a.server.Serve(listener) }()
	a.initialized = true
	return nil
}

// credentialsFile is the shared credentials file the app reads. It holds the
// ambient identity when there is one, and nothing otherwise, so a developer's
// own ~/.aws/credentials is never what the app finds.
func (a *AWS) credentialsFile() string {
	if !a.ambient {
		return "# written by tomato; no ambient identity (ambient_identity: false)\n"
	}
	creds := awsid.Ambient(a.name)
	return "# written by tomato: the ambient identity, what a credential chain falls back to when the role session\n" +
		"# cannot be had (an EKS node role, say)\n" +
		"[default]\n" +
		"aws_access_key_id = " + creds.AccessKeyID + "\n" +
		"aws_secret_access_key = " + creds.SecretAccessKey + "\n"
}

// URL is the STS endpoint.
func (a *AWS) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", a.port)
}

// AppEnv points the app's AWS SDK at the IRSA role and tomato's STS, and keeps
// credentials from tomato's own environment out of the app's.
func (a *AWS) AppEnv() resource.AppEnv {
	return resource.AppEnv{
		Set: map[string]string{
			"AWS_REGION":                  a.region,
			"AWS_DEFAULT_REGION":          a.region,
			"AWS_ROLE_ARN":                a.roleARN,
			"AWS_ROLE_SESSION_NAME":       a.sessionName,
			"AWS_WEB_IDENTITY_TOKEN_FILE": filepath.Join(a.dir, "web-identity-token"),
			"AWS_ENDPOINT_URL_STS":        a.URL(),
			"AWS_SHARED_CREDENTIALS_FILE": filepath.Join(a.dir, "credentials"),
			"AWS_CONFIG_FILE":             filepath.Join(a.dir, "config"),
			"AWS_EC2_METADATA_DISABLED":   "true",
		},
		Unset: []string{
			"AWS_ACCESS_KEY_ID",
			"AWS_SECRET_ACCESS_KEY",
			"AWS_SESSION_TOKEN",
			"AWS_PROFILE",
			"AWS_DEFAULT_PROFILE",
			"AWS_ENDPOINT_URL",
			"AWS_CONTAINER_CREDENTIALS_FULL_URI",
			"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI",
			"AWS_CONTAINER_AUTHORIZATION_TOKEN",
			"AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE",
		},
	}
}

const stsNamespace = "https://sts.amazonaws.com/doc/2011-06-15/"

// accessKeyInAuthorization pulls the access key id out of a SigV4 Authorization
// header ("... Credential=<key>/<date>/<region>/sts/aws4_request, ...").
var accessKeyInAuthorization = regexp.MustCompile(`Credential=([^/,\s]+)/`)

func (a *AWS) serveSTS(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		writeSTSError(w, http.StatusBadRequest, "InvalidParameterValue", err.Error())
		return
	}
	switch action := req.Form.Get("Action"); action {
	case "AssumeRoleWithWebIdentity", "AssumeRole":
		role := req.Form.Get("RoleArn")
		if role == "" {
			writeSTSError(w, http.StatusBadRequest, "MissingParameter", "RoleArn is required")
			return
		}
		if action == "AssumeRoleWithWebIdentity" && req.Form.Get("WebIdentityToken") == "" {
			writeSTSError(w, http.StatusBadRequest, "MissingParameter", "WebIdentityToken is required")
			return
		}
		session := req.Form.Get("RoleSessionName")
		if session == "" {
			session = a.sessionName
		}
		a.mu.Lock()
		a.assumed = append(a.assumed, awsAssumption{Action: action, RoleARN: role, Session: session, At: time.Now()})
		a.mu.Unlock()
		writeXML(w, assumeRoleResponse(action, role, session))
	case "GetCallerIdentity":
		key := ""
		if m := accessKeyInAuthorization.FindStringSubmatch(req.Header.Get("Authorization")); m != nil {
			key = m[1]
		}
		writeXML(w, a.callerIdentity(key))
	default:
		writeSTSError(w, http.StatusBadRequest, "InvalidAction", fmt.Sprintf("tomato's STS does not implement %q", action))
	}
}

type stsCredentials struct {
	AccessKeyID     string `xml:"AccessKeyId"`
	SecretAccessKey string `xml:"SecretAccessKey"`
	SessionToken    string `xml:"SessionToken"`
	Expiration      string `xml:"Expiration"`
}

type stsAssumedRoleUser struct {
	Arn           string `xml:"Arn"`
	AssumedRoleID string `xml:"AssumedRoleId"`
}

type stsAssumeRoleResult struct {
	SubjectFromWebIdentityToken string             `xml:"SubjectFromWebIdentityToken,omitempty"`
	Audience                    string             `xml:"Audience,omitempty"`
	AssumedRoleUser             stsAssumedRoleUser `xml:"AssumedRoleUser"`
	Credentials                 stsCredentials     `xml:"Credentials"`
	Provider                    string             `xml:"Provider,omitempty"`
}

type stsResponseMetadata struct {
	RequestID string `xml:"RequestId"`
}

type stsAssumeRoleWithWebIdentityResponse struct {
	XMLName  xml.Name            `xml:"AssumeRoleWithWebIdentityResponse"`
	Xmlns    string              `xml:"xmlns,attr"`
	Result   stsAssumeRoleResult `xml:"AssumeRoleWithWebIdentityResult"`
	Metadata stsResponseMetadata `xml:"ResponseMetadata"`
}

type stsAssumeRoleResponse struct {
	XMLName  xml.Name            `xml:"AssumeRoleResponse"`
	Xmlns    string              `xml:"xmlns,attr"`
	Result   stsAssumeRoleResult `xml:"AssumeRoleResult"`
	Metadata stsResponseMetadata `xml:"ResponseMetadata"`
}

type stsGetCallerIdentityResponse struct {
	XMLName xml.Name `xml:"GetCallerIdentityResponse"`
	Xmlns   string   `xml:"xmlns,attr"`
	Result  struct {
		Arn     string `xml:"Arn"`
		UserID  string `xml:"UserId"`
		Account string `xml:"Account"`
	} `xml:"GetCallerIdentityResult"`
	Metadata stsResponseMetadata `xml:"ResponseMetadata"`
}

type stsErrorResponse struct {
	XMLName xml.Name `xml:"ErrorResponse"`
	Xmlns   string   `xml:"xmlns,attr"`
	Error   struct {
		Type    string `xml:"Type"`
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	} `xml:"Error"`
	RequestID string `xml:"RequestId"`
}

func assumeRoleResponse(action, role, session string) any {
	creds := awsid.Session(role)
	account, roleName := splitRoleARN(role)
	result := stsAssumeRoleResult{
		AssumedRoleUser: stsAssumedRoleUser{
			Arn:           fmt.Sprintf("arn:aws:sts::%s:assumed-role/%s/%s", account, roleName, session),
			AssumedRoleID: "TOMATOROLE" + strings.TrimPrefix(creds.AccessKeyID, "TOMATO") + ":" + session,
		},
		Credentials: stsCredentials{
			AccessKeyID:     creds.AccessKeyID,
			SecretAccessKey: creds.SecretAccessKey,
			SessionToken:    creds.SessionToken,
			// Long enough that no SDK refreshes during a run.
			Expiration: time.Now().UTC().Add(12 * time.Hour).Format(time.RFC3339),
		},
	}
	meta := stsResponseMetadata{RequestID: uuid.NewString()}
	if action == "AssumeRole" {
		return stsAssumeRoleResponse{Xmlns: stsNamespace, Result: result, Metadata: meta}
	}
	result.SubjectFromWebIdentityToken = "system:serviceaccount:tomato:" + session
	result.Audience = "sts.amazonaws.com"
	result.Provider = "tomato"
	return stsAssumeRoleWithWebIdentityResponse{Xmlns: stsNamespace, Result: result, Metadata: meta}
}

// callerIdentity answers GetCallerIdentity for the access key that signed the
// request: the resource's role session, its ambient identity, or unknown.
func (a *AWS) callerIdentity(accessKeyID string) stsGetCallerIdentityResponse {
	account, roleName := splitRoleARN(a.roleARN)
	resp := stsGetCallerIdentityResponse{Xmlns: stsNamespace, Metadata: stsResponseMetadata{RequestID: uuid.NewString()}}
	resp.Result.Account = account
	switch accessKeyID {
	case awsid.Session(a.roleARN).AccessKeyID:
		resp.Result.Arn = fmt.Sprintf("arn:aws:sts::%s:assumed-role/%s/%s", account, roleName, a.sessionName)
		resp.Result.UserID = accessKeyID + ":" + a.sessionName
	case awsid.Ambient(a.name).AccessKeyID:
		resp.Result.Arn = fmt.Sprintf("arn:aws:iam::%s:user/tomato-ambient", account)
		resp.Result.UserID = accessKeyID
	default:
		resp.Result.Arn = fmt.Sprintf("arn:aws:iam::%s:user/unknown", account)
		resp.Result.UserID = accessKeyID
	}
	return resp
}

// splitRoleARN returns the account and role name of arn:aws:iam::<account>:role/<path/><name>.
func splitRoleARN(arn string) (account, name string) {
	account = "000000000000"
	name = arn
	if parts := strings.SplitN(arn, ":", 6); len(parts) == 6 {
		if parts[4] != "" {
			account = parts[4]
		}
		resource := parts[5]
		name = resource[strings.LastIndex(resource, "/")+1:]
	}
	return account, name
}

func writeXML(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml")
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

func writeSTSError(w http.ResponseWriter, status int, code, message string) {
	resp := stsErrorResponse{Xmlns: stsNamespace, RequestID: uuid.NewString()}
	resp.Error.Type = "Sender"
	resp.Error.Code = code
	resp.Error.Message = message
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(resp)
}

func (a *AWS) Ready(ctx context.Context) error { return nil }

// Reset keeps the journal: roles are assumed when the app starts, before the
// first scenario, and a reset would erase exactly that.
func (a *AWS) Reset(ctx context.Context) error { return nil }

func (a *AWS) RegisterSteps(ctx *godog.ScenarioContext) {
	resource.RegisterStepsToGodog(ctx, a.name, a.Steps())
}

// Steps returns the structured step definitions for the aws resource.
func (a *AWS) Steps() resource.StepCategory {
	return resource.StepCategory{
		Name:        "AWS",
		Description: "Steps for the AWS identity (IRSA) tomato gives the application: which roles it assumed at tomato's STS",
		Steps: []resource.StepDef{
			{
				Group:       "Identity",
				Pattern:     `^"{resource}" role "([^"]*)" was assumed$`,
				Description: "Asserts the application assumed the role at tomato's STS during the run",
				Example:     `"{resource}" role "arn:aws:iam::000000000000:role/my-service" was assumed`,
				Handler:     a.roleWasAssumed,
			},
			{
				Group:       "Identity",
				Pattern:     `^"{resource}" role "([^"]*)" was not assumed$`,
				Description: "Asserts the application never assumed the role at tomato's STS during the run",
				Example:     `"{resource}" role "arn:aws:iam::000000000000:role/my-service" was not assumed`,
				Handler:     a.roleWasNotAssumed,
			},
		},
	}
}

func (a *AWS) assumedCount(role string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, as := range a.assumed {
		if as.RoleARN == role {
			n++
		}
	}
	return n
}

func (a *AWS) roleWasAssumed(role string) error {
	if a.assumedCount(role) == 0 {
		return fmt.Errorf("role %q was never assumed at %s (assumed: %s)", role, a.URL(), a.assumedRoles())
	}
	return nil
}

func (a *AWS) roleWasNotAssumed(role string) error {
	if n := a.assumedCount(role); n > 0 {
		return fmt.Errorf("role %q was assumed %d time(s)", role, n)
	}
	return nil
}

func (a *AWS) assumedRoles() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.assumed) == 0 {
		return "none"
	}
	seen := make(map[string]bool)
	var roles []string
	for _, as := range a.assumed {
		if !seen[as.RoleARN] {
			seen[as.RoleARN] = true
			roles = append(roles, as.RoleARN)
		}
	}
	return strings.Join(roles, ", ")
}

func (a *AWS) Cleanup(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = a.server.Shutdown(shutdownCtx)
	}
	if a.dir != "" {
		_ = os.RemoveAll(a.dir)
	}
	return nil
}
