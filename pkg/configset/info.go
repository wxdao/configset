package configset

import "context"

type SetInfo struct {
	Name      string         `json:"name"`
	Resources []ResourceInfo `json:"resources"`
	UpdatedAt string         `json:"updatedAt"`
}

type ResourceInfo struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
}

type SetInfoStore interface {
	GetSetInfo(ctx context.Context, name string) (*SetInfo, error)
	ListSetInfos(ctx context.Context) ([]*SetInfo, error)
	CreateSetInfo(ctx context.Context, name string, info *SetInfo) error
	UpdateSetInfo(ctx context.Context, name string, info *SetInfo) error
	DeleteSetInfo(ctx context.Context, name string) error
}
