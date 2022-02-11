package configset

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultSetInfoSecretPrefix            = "configset.v1."
	DefaultSetInfoSecretDataKey           = "data"
	DefaultSetInfoSecretFieldOwner        = "configset/secret-store"
	DefaultSetInfoSecretLockAnnotationKey = "configset/lock-id"
	DefaultSetInfoSecretIsSetInfoLabelKey = "configset/is-set-info"
)

type SecretSetInfoStore struct {
	kube              crclient.Client
	namespace         string
	namePrefix        string
	dataKey           string
	fieldOwner        string
	lockAnnoKey       string
	isSetInfoLabelKey string
}

var _ SetInfoStore = &SecretSetInfoStore{}

func NewSecretSetInfoStore(restConfig *rest.Config, namespace string) (*SecretSetInfoStore, error) {
	kube, err := crclient.New(restConfig, crclient.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	return &SecretSetInfoStore{
		kube:              kube,
		namespace:         namespace,
		namePrefix:        DefaultSetInfoSecretPrefix,
		dataKey:           DefaultSetInfoSecretDataKey,
		fieldOwner:        DefaultSetInfoSecretFieldOwner,
		lockAnnoKey:       DefaultSetInfoSecretLockAnnotationKey,
		isSetInfoLabelKey: DefaultSetInfoSecretIsSetInfoLabelKey,
	}, nil
}

func (s *SecretSetInfoStore) GetSetInfo(ctx context.Context, name string) (*SetInfo, error) {
	var secret corev1.Secret
	if err := s.kube.Get(ctx, types.NamespacedName{Namespace: s.namespace, Name: s.namePrefix + name}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	return setInfoFromJSON(secret.Data[s.dataKey])
}

func (s *SecretSetInfoStore) ListSetInfos(ctx context.Context) ([]*SetInfo, error) {
	var secretList corev1.SecretList
	if err := s.kube.List(ctx, &secretList, crclient.InNamespace(s.namespace), crclient.HasLabels{s.isSetInfoLabelKey}); err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}
	infos := make([]*SetInfo, 0, len(secretList.Items))
	for _, secret := range secretList.Items {
		info, err := setInfoFromJSON(secret.Data[s.dataKey])
		if err != nil {
			return nil, fmt.Errorf("failed to parse secret %s: %w", secret.Name, err)
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (s *SecretSetInfoStore) CreateSetInfo(ctx context.Context, name string, info *SetInfo) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.namespace,
			Name:      s.namePrefix + name,
			Labels: map[string]string{
				s.isSetInfoLabelKey: "true",
			},
		},
		Data: map[string][]byte{
			s.dataKey: info.toJSON(),
		},
	}
	if err := s.kube.Create(ctx, secret, crclient.FieldOwner(s.fieldOwner)); err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}
	return nil
}

func (s *SecretSetInfoStore) UpdateSetInfo(ctx context.Context, name string, info *SetInfo) error {
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.namespace,
			Name:      s.namePrefix + name,
			Labels: map[string]string{
				s.isSetInfoLabelKey: "true",
			},
		},
		Data: map[string][]byte{s.dataKey: info.toJSON()},
	}
	secret.GetObjectKind()
	if err := s.kube.Patch(ctx, &secret, crclient.Apply, crclient.FieldOwner(s.fieldOwner), crclient.ForceOwnership); err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}
	return nil
}

func (s *SecretSetInfoStore) DeleteSetInfo(ctx context.Context, name string) error {
	if err := crclient.IgnoreNotFound(s.kube.Delete(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.namespace,
			Name:      s.namePrefix + name,
		},
	})); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	return nil
}

func setInfoFromJSON(b []byte) (*SetInfo, error) {
	var info SetInfo
	if err := json.Unmarshal(b, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (i SetInfo) toJSON() []byte {
	b, _ := json.Marshal(i)
	return b
}
