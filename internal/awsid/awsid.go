// Package awsid derives the AWS identities tomato hands out: the session
// credentials the aws resource's STS issues for a role, and the ambient identity
// it can leave in a shared credentials file. They are derived, not random, so the
// aws resource and the kafka preset (whose AWS_MSK_IAM listener lets only role
// sessions in) agree on them without talking to each other, and a run is
// reproducible.
package awsid

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// DefaultRoleARN is the role an aws resource hands out when it configures none.
const DefaultRoleARN = "arn:aws:iam::000000000000:role/tomato"

// Credentials is one AWS identity.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	// SessionToken is empty for a long-term identity.
	SessionToken string
}

// Session returns the credentials STS issues for roleARN. They are unique per
// role, so a service that ends up with another identity can be told apart. The
// access key id is 20 characters like a real one, without the AKIA/ASIA prefix
// secret scanners look for.
func Session(roleARN string) Credentials {
	h := digest("session:" + roleARN)
	return Credentials{
		AccessKeyID:     "TOMATO" + strings.ToUpper(h[:14]),
		SecretAccessKey: "tomato-secret-" + h[14:46],
		SessionToken:    "tomato-session-" + h[:32],
	}
}

// Ambient returns the long-term identity an aws resource named resource leaves
// in its shared credentials file: what an SDK's credential chain falls back to
// when the role session cannot be had, like an EKS node role. It is never a
// role session, so wherever only role sessions are allowed, it is not.
func Ambient(resource string) Credentials {
	h := digest("ambient:" + resource)
	return Credentials{
		AccessKeyID:     "TOMATOAMB" + strings.ToUpper(h[:11]),
		SecretAccessKey: "tomato-ambient-" + h[11:43],
	}
}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
