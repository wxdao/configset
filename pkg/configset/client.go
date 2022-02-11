package configset

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultFieldOwner = "configset"
)

type Object interface {
	metav1.Object
	runtime.Object
}

type Client struct {
	kube       crclient.Client
	store      SetInfoStore
	fieldOwner string
}

func NewClient(restConfig *rest.Config, store SetInfoStore) (*Client, error) {
	kube, err := crclient.New(restConfig, crclient.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	return &Client{
		kube:       kube,
		store:      store,
		fieldOwner: DefaultFieldOwner,
	}, nil
}

func (c *Client) Store() SetInfoStore {
	return c.store
}

// common types

type ObjectAction string

const (
	ObjectActionUpdate    ObjectAction = "update"
	LogObjectActionDelete ObjectAction = "delete"
)

type ObjectResult struct {
	Action ObjectAction
	Error  error

	Config  Object
	Live    Object
	Updated Object
}

var (
	ErrFailedToOperateSomeResources = fmt.Errorf("failed to opearate some resources")
)

// apply

type ApplyOptions struct {
	DryRun              bool
	ForceConflicts      bool
	LogObjectResultFunc func(ObjectResult)
}

type ApplyResult struct {
	ObjectResults []ObjectResult
}

func (c *Client) Apply(ctx context.Context, name string, objs []Object, opt ApplyOptions) (ApplyResult, error) {
	var res ApplyResult

	if opt.LogObjectResultFunc == nil {
		opt.LogObjectResultFunc = func(or ObjectResult) {}
	}

	updatedSetInfo := &SetInfo{
		Name:      name,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	updatedUIDs := map[string]struct{}{}
	patchOpts := []crclient.PatchOption{crclient.FieldOwner(c.fieldOwner)}
	if opt.DryRun {
		patchOpts = append(patchOpts, crclient.DryRunAll)
	}
	if opt.ForceConflicts {
		patchOpts = append(patchOpts, crclient.ForceOwnership)
	}
	hasErrors := false
	for _, obj := range objs {
		objRes := ObjectResult{
			Action: ObjectActionUpdate,
			Config: obj.DeepCopyObject().(Object),
		}

		var liveObj unstructured.Unstructured
		liveObj.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
		err := c.kube.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}, &liveObj)
		if apierrors.IsNotFound(err) {
			objRes.Live = nil
		} else if err != nil {
			hasErrors = true
			objRes.Error = fmt.Errorf("failed to get live object: %w", err)
			res.ObjectResults = append(res.ObjectResults, objRes)
			opt.LogObjectResultFunc(objRes)
			continue
		} else {
			objRes.Live = &liveObj
		}

		if err := c.kube.Patch(ctx, obj, crclient.Apply, patchOpts...); err != nil {
			hasErrors = true
			objRes.Error = fmt.Errorf("failed to apply object: %w", err)
			res.ObjectResults = append(res.ObjectResults, objRes)
			opt.LogObjectResultFunc(objRes)
			continue
		}
		objRes.Updated = obj
		res.ObjectResults = append(res.ObjectResults, objRes)

		gvk := obj.GetObjectKind().GroupVersionKind()
		apiVersion := gvk.Group + "/" + gvk.Version
		if gvk.Group == "" {
			apiVersion = gvk.Version
		}
		updatedSetInfo.Resources = append(updatedSetInfo.Resources, ResourceInfo{
			APIVersion: apiVersion,
			Kind:       gvk.Kind,
			Namespace:  obj.GetNamespace(),
			Name:       obj.GetName(),
			UID:        string(obj.GetUID()),
		})
		updatedUIDs[string(obj.GetUID())] = struct{}{}
		opt.LogObjectResultFunc(objRes)
	}

	liveSetInfo, err := c.store.GetSetInfo(ctx, name)
	if err != nil {
		return res, fmt.Errorf("failed to get set info: %w", err)
	}
	if liveSetInfo == nil {
		liveSetInfo = &SetInfo{Name: name}
	}

	// prune resources
	updatedSetInfoWithLiveMerged := *updatedSetInfo
	toPrune := []ResourceInfo{}
	for _, r := range liveSetInfo.Resources {
		if _, ok := updatedUIDs[r.UID]; !ok {
			updatedSetInfoWithLiveMerged.Resources = append(updatedSetInfoWithLiveMerged.Resources, r)
			if !hasErrors {
				// not to run prune logic if there were any errors on applying
				toPrune = append(toPrune, r)
			}
		}
	}
	// prune in reverse order
	deleteOpts := []crclient.DeleteOption{}
	if opt.DryRun {
		deleteOpts = append(deleteOpts, crclient.DryRunAll)
	}
	for i := len(toPrune) - 1; i >= 0; i-- {
		info := toPrune[i]

		obj := unstructured.Unstructured{}
		obj.SetAPIVersion(info.APIVersion)
		obj.SetKind(info.Kind)
		obj.SetNamespace(info.Namespace)
		obj.SetName(info.Name)

		objRes := ObjectResult{
			Action: LogObjectActionDelete,
			Config: obj.DeepCopy(),
		}

		var liveObj unstructured.Unstructured
		liveObj.SetAPIVersion(info.APIVersion)
		liveObj.SetKind(info.Kind)
		err := c.kube.Get(ctx, types.NamespacedName{Namespace: info.Namespace, Name: info.Name}, &liveObj)
		if apierrors.IsNotFound(err) {
			objRes.Live = nil
		} else if err != nil {
			hasErrors = true
			objRes.Error = fmt.Errorf("failed to get live object: %w", err)
			res.ObjectResults = append(res.ObjectResults, objRes)
			opt.LogObjectResultFunc(objRes)
			continue
		} else {
			objRes.Live = &liveObj
		}

		if err := crclient.IgnoreNotFound(c.kube.Delete(ctx, &obj, deleteOpts...)); err != nil {
			hasErrors = true
			objRes.Error = fmt.Errorf("failed to delete object: %w", err)
			res.ObjectResults = append(res.ObjectResults, objRes)
			opt.LogObjectResultFunc(objRes)
			continue
		}
		objRes.Updated = nil
		res.ObjectResults = append(res.ObjectResults, objRes)
		opt.LogObjectResultFunc(objRes)
	}

	if hasErrors {
		// not to forget previous resources if any errors occurred
		// so that next retry will hopefully catch up what's left
		updatedSetInfo = &updatedSetInfoWithLiveMerged
	}

	if !opt.DryRun {
		if err := c.store.UpdateSetInfo(ctx, name, updatedSetInfo); err != nil {
			return res, fmt.Errorf("failed to update set info: %w", err)
		}
	}

	if hasErrors {
		return res, ErrFailedToOperateSomeResources
	}

	return res, nil
}

// delete

type DeleteOptions struct {
	DryRun              bool
	LogObjectResultFunc func(ObjectResult)
}

type DeleteResult struct {
	ObjectResults []ObjectResult
}

func (c *Client) Delete(ctx context.Context, name string, opt DeleteOptions) (DeleteResult, error) {
	var res DeleteResult

	liveSetInfo, err := c.store.GetSetInfo(ctx, name)
	if err != nil {
		return res, fmt.Errorf("failed to get set info: %w", err)
	}
	if liveSetInfo == nil {
		liveSetInfo = &SetInfo{Name: name}
	}

	deleteOpts := []crclient.DeleteOption{}
	if opt.DryRun {
		deleteOpts = append(deleteOpts, crclient.DryRunAll)
	}
	hasErrors := false
	// delete in reverse order
	for i := len(liveSetInfo.Resources) - 1; i >= 0; i-- {
		info := liveSetInfo.Resources[i]

		obj := unstructured.Unstructured{}
		obj.SetAPIVersion(info.APIVersion)
		obj.SetKind(info.Kind)
		obj.SetNamespace(info.Namespace)
		obj.SetName(info.Name)

		objRes := ObjectResult{
			Action: LogObjectActionDelete,
			Config: obj.DeepCopy(),
		}

		var liveObj unstructured.Unstructured
		liveObj.SetAPIVersion(info.APIVersion)
		liveObj.SetKind(info.Kind)
		err := c.kube.Get(ctx, types.NamespacedName{Namespace: info.Namespace, Name: info.Name}, &liveObj)
		if apierrors.IsNotFound(err) {
			objRes.Live = nil
		} else if err != nil {
			hasErrors = true
			objRes.Error = fmt.Errorf("failed to get live object: %w", err)
			res.ObjectResults = append(res.ObjectResults, objRes)
			opt.LogObjectResultFunc(objRes)
			continue
		} else {
			objRes.Live = &liveObj
		}

		if err := crclient.IgnoreNotFound(c.kube.Delete(ctx, &obj, deleteOpts...)); err != nil {
			hasErrors = true
			objRes.Error = fmt.Errorf("failed to delete object: %w", err)
			res.ObjectResults = append(res.ObjectResults, objRes)
			opt.LogObjectResultFunc(objRes)
			continue
		}
		objRes.Updated = nil
		res.ObjectResults = append(res.ObjectResults, objRes)
		opt.LogObjectResultFunc(objRes)
	}

	if !hasErrors && !opt.DryRun {
		if err := c.store.DeleteSetInfo(ctx, name); err != nil {
			return res, fmt.Errorf("failed to delete set info: %w", err)
		}
	}

	if hasErrors {

		return res, ErrFailedToOperateSomeResources
	}

	return res, nil
}
