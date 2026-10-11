package handler

import "github.com/tomatool/tomato/internal/resource"

// This file is scaffolding for the move of each resource into its own package
// under internal/resource. It lets the resources that have not moved yet keep
// referring to the contract by its old unqualified names. It shrinks with
// every resource that moves out, and goes away with the last one.

type (
	Handler                  = resource.Handler
	AppEnv                   = resource.AppEnv
	AppEnvProvider           = resource.AppEnvProvider
	SQLExecutor              = resource.SQLExecutor
	MessagePublisher         = resource.MessagePublisher
	MessageConsumer          = resource.MessageConsumer
	CacheStore               = resource.CacheStore
	WebSocketClientInterface = resource.WebSocketClientInterface
	StepDef                  = resource.StepDef
	StepCategory             = resource.StepCategory
	StepRegistry             = resource.StepRegistry
	StepProvider             = resource.StepProvider
	Variables                = resource.Variables
)

var (
	NewStepRegistry      = resource.NewStepRegistry
	RegisterStepsToGodog = resource.RegisterStepsToGodog
	FormatStepPattern    = resource.FormatStepPattern
	FormatStepExample    = resource.FormatStepExample
	DummyConfig          = resource.DummyConfig
	NewVariables         = resource.NewVariables
	GetGlobalVariables   = resource.GetGlobalVariables
	ResetGlobalVariables = resource.ResetGlobalVariables
	SetVariable          = resource.SetVariable
	GetVariable          = resource.GetVariable
	ReplaceVariables     = resource.ReplaceVariables
)
