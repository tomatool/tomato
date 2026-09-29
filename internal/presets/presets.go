// Package presets holds the files tomato puts into preset containers.
package presets

import _ "embed"

// MskIamJar is the AWS_MSK_IAM SASL server for Kafka brokers in kafka/src,
// compiled. The kafka preset copies it into its apache/kafka container when
// auth is aws_msk_iam, so the preset needs no image of its own.
//
// It is committed because Go can only embed files in the module, and tomato
// has to build without a JDK. `make kafka-plugin` rebuilds it from kafka/src;
// CI rebuilds it and fails when the committed jar differs.
//
//go:embed kafka/tomato-msk-iam.jar
var MskIamJar []byte
