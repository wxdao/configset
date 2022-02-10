package configset

import (
	"fmt"

	"github.com/wxdao/configset/pkg/diffutil"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/yaml"
)

func WriteObjectResultsToDiffer(results []ObjectResult, differ *diffutil.Differ, prefix string) error {
	filename := func(result ObjectResult) string {
		obj := result.Live
		if obj == nil {
			obj = result.Updated
		}

		gvk := obj.GetObjectKind().GroupVersionKind()

		return fmt.Sprintf(
			"%s%s_%s_%s_%s_%s.yaml",
			prefix,
			obj.GetNamespace(),
			obj.GetName(),
			gvk.Group,
			gvk.Version,
			gvk.Kind,
		)
	}

	for _, result := range results {
		if result.Error != nil || (result.Live == nil && result.Updated == nil) {
			continue
		}

		if result.Live != nil {
			obj := result.Live.DeepCopyObject()
			ac, err := meta.Accessor(obj)
			if err != nil {
				return fmt.Errorf("failed to get accessor for object: %w", err)
			}
			ac.SetManagedFields(nil)

			b, err := yaml.Marshal(obj)
			if err != nil {
				return err
			}
			if err := differ.AddOld(filename(result), b); err != nil {
				return err
			}
		}
		if result.Updated != nil {
			obj := result.Updated.DeepCopyObject()
			ac, err := meta.Accessor(obj)
			if err != nil {
				return fmt.Errorf("failed to get accessor for object: %w", err)
			}
			ac.SetManagedFields(nil)

			b, err := yaml.Marshal(obj)
			if err != nil {
				return err
			}
			if err := differ.AddNew(filename(result), b); err != nil {
				return err
			}
		}
	}

	return nil
}
