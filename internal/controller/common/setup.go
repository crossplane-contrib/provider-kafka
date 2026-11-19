/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
)

// SetupManaged adds a controller that reconciles managed resources of kind
// gvk. obj and list are empty instances of the kind and its list type. extra
// options come after the defaults every controller shares.
func SetupManaged(mgr ctrl.Manager, o controller.Options, gvk schema.GroupVersionKind, obj resource.Managed, list resource.ManagedList,
	ec managed.ExternalConnector, extra ...managed.ReconcilerOption,
) error {
	name := managed.ControllerName(gvk.GroupKind().String())

	opts := append([]managed.ReconcilerOption{
		managed.WithExternalConnector(ec),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))), //nolint:staticcheck // crossplane-runtime doesn't support new events API yet
	}, extra...)

	if o.Features.Enabled(feature.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}

	if o.Features.Enabled(feature.EnableAlphaChangeLogs) {
		opts = append(opts, managed.WithChangeLogger(o.ChangeLogOptions.ChangeLogger))
	}

	if o.MetricOptions != nil {
		opts = append(opts, managed.WithMetricRecorder(o.MetricOptions.MRMetrics))
	}

	if o.MetricOptions != nil && o.MetricOptions.MRStateMetrics != nil {
		stateMetricsRecorder := statemetrics.NewMRStateRecorder(
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, list, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return fmt.Errorf("cannot register MR state metrics recorder for kind %sList: %w", gvk.Kind, err)
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(gvk), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(obj).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// SetupGated registers setup to run once the CRD for gvk is available.
func SetupGated(mgr ctrl.Manager, o controller.Options, gvk schema.GroupVersionKind, setup func(ctrl.Manager, controller.Options) error) error {
	o.Gate.Register(func() {
		if err := setup(mgr, o); err != nil {
			panic(fmt.Errorf("cannot setup %s controller: %w", gvk.Kind, err))
		}
	}, gvk)
	return nil
}
