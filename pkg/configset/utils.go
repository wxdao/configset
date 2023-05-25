package configset

import (
	"encoding/json"
	"fmt"

	"github.com/samber/lo"
	"github.com/wxdao/configset/pkg/diffutil"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

type AddObjectResultsToDifferOptions struct {
	Prefix                                       string
	StripManagedFields                           bool
	StripGeneration                              bool
	FixAutoscalingV2Beta2HorizontalPodAutoscaler bool
}

func AddObjectResultsToDiffer(results []ObjectResult, differ *diffutil.Differ, opt AddObjectResultsToDifferOptions) error {
	filename := func(result ObjectResult) string {
		obj := result.Live
		if obj == nil {
			obj = result.Updated
		}

		gvk := obj.GetObjectKind().GroupVersionKind()

		return fmt.Sprintf(
			"%s%s_%s_%s_%s_%s.yaml",
			opt.Prefix,
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
			if opt.StripManagedFields {
				ac.SetManagedFields(nil)
			}
			if opt.StripGeneration {
				ac.SetGeneration(0)
			}

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
			if opt.StripManagedFields {
				ac.SetManagedFields(nil)
			}
			if opt.StripGeneration {
				ac.SetGeneration(0)
			}

			if opt.FixAutoscalingV2Beta2HorizontalPodAutoscaler && obj.GetObjectKind().GroupVersionKind() == schema.FromAPIVersionAndKind("autoscaling/v2beta2", "HorizontalPodAutoscaler") {
				un := obj.(*unstructured.Unstructured)
				uc := un.UnstructuredContent()
				spec, ok := uc["spec"].(map[string]interface{})
				if ok {
					metrics, ok := spec["metrics"].([]interface{})
					if ok {
						newMetrics := lo.UniqBy(metrics, func(item interface{}) string {
							b, _ := json.Marshal(item)
							return string(b)
						})

						spec["metrics"] = newMetrics
						uc["spec"] = spec
						un.SetUnstructuredContent(uc)
						obj = un
					}
				}
			}

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
