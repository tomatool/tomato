package composer

import (
	"context"

	"github.com/tomatool/tomato/config"
)

type Composer interface {
	CreateContainer(ctx context.Context, config *config.Resource) error
	DeleteAll(ctx context.Context) error
}

const (
	ComposerTypeDocker     = "docker"
	ComposerTypeKubernetes = "kubernetes"
)

type DefaultComposer struct{}

func (*DefaultComposer) CreateContainer(ctx context.Context, config *config.Resource) error {
	return nil
}
func (*DefaultComposer) DeleteAll(ctx context.Context) error {
	return nil
}
