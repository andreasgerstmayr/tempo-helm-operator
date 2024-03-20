package controller

import (
	"fmt"
	"io"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	k8sjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *TempoMicroservicesReconciler) renderHelmChart(chart *chart.Chart, obj client.Object, vals chartutil.Values) ([]client.Object, error) {
	actionClient, err := r.ActionClientGetter.ActionClientFor(obj)
	if err != nil {
		return nil, err
	}

	dryRunOpts := func(i *action.Install) error {
		i.DryRun = true
		i.IsUpgrade = true
		return nil
	}
	rel, err := actionClient.Install(obj.GetName(), obj.GetNamespace(), chart, vals, dryRunOpts)
	if err != nil {
		return nil, err
	}

	reader := io.NopCloser(strings.NewReader(rel.Manifest))
	yamlStreamReader := k8sjson.YAMLFramer.NewFrameReader(reader)
	decoder := streaming.NewDecoder(yamlStreamReader, scheme.Codecs.UniversalDeserializer())
	manifests := []client.Object{}
	for {
		obj, _, err := decoder.Decode(nil, nil)
		if err == io.EOF {
			break
		}

		switch t := obj.(type) {
		case client.Object:
			manifests = append(manifests, t)
		default:
			return nil, fmt.Errorf("invalid object: %v", t)
		}
	}

	return manifests, nil
}
