/*
Copyright 2020 The Crossplane Authors.

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

package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/alecthomas/kong"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	authv1 "k8s.io/api/authorization/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	changelogsv1alpha1 "github.com/crossplane/crossplane-runtime/v2/apis/changelogs/proto/v1alpha1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/gate"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/customresourcesgate"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	clusterapis "github.com/crossplane-contrib/provider-kafka/apis/cluster"
	namespacedapis "github.com/crossplane-contrib/provider-kafka/apis/namespaced"

	clustercontroller "github.com/crossplane-contrib/provider-kafka/internal/controller/cluster"
	namespacedcontroller "github.com/crossplane-contrib/provider-kafka/internal/controller/namespaced"
	"github.com/crossplane-contrib/provider-kafka/internal/version"
)

var cli struct {
	Debug          bool `help:"Run with debug logging." short:"d"`
	LeaderElection bool `help:"Use leader election for the controller manager." short:"l" default:"false" env:"LEADER_ELECTION"`

	SyncPeriod              time.Duration `help:"Controller manager sync period such as 300ms, 1.5h, or 2h45m" short:"s" default:"1h"`
	PollInterval            time.Duration `help:"How often individual resources will be checked for drift from the desired state" default:"1m"`
	PollStateMetricInterval time.Duration `help:"State metric recording interval" default:"5s"`

	MaxReconcileRate         int    `help:"The global maximum rate per second at which resources may checked for drift from the desired state." default:"10"`
	EnableManagementPolicies bool   `help:"Enable support for Management Policies." default:"true" env:"ENABLE_MANAGEMENT_POLICIES"`
	EnableChangeLogs         bool   `help:"Enable support for capturing change logs during reconciliation." default:"false" env:"ENABLE_CHANGE_LOGS"`
	ChangelogsSocketPath     string `help:"Path for changelogs socket (if enabled)" default:"/var/run/changelogs/changelogs.sock" env:"CHANGELOGS_SOCKET_PATH"`

	BrokerConnectionTimeout time.Duration `help:"Timeout for establishing connection to Kafka brokers" default:"30s"`
}

func main() {
	ctx := kong.Parse(&cli, kong.Description("Crossplane Kafka Provider"))

	zl := zap.New(zap.UseDevMode(cli.Debug))
	log := logging.NewLogrLogger(zl.WithName("provider-kafka"))
	if cli.Debug {
		// The controller-runtime runs with a no-op logger by default. It is
		// *very* verbose even at info level, so we only provide it a real
		// logger when we're running in debug mode.
		ctrl.SetLogger(zl)
	} else {
		// Setting the controller-runtime logger to a no-op logger by default. This
		// is not really needed, but otherwise we get a warning from the
		// controller-runtime.
		ctrl.SetLogger(zap.New(zap.WriteTo(io.Discard)))
	}
	ctx.Bind(log)

	cfg, err := ctrl.GetConfig()
	ctx.FatalIfErrorf(err, "Cannot get API server rest config")

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:             cli.LeaderElection,
		LeaderElectionID:           "crossplane-leader-election-provider-kafka",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaseDuration:              func() *time.Duration { d := 60 * time.Second; return &d }(),
		RenewDeadline:              func() *time.Duration { d := 50 * time.Second; return &d }(),
		Cache: cache.Options{
			SyncPeriod: &cli.SyncPeriod,
		},
	})
	ctx.FatalIfErrorf(err, "Cannot create controller manager")
	ctx.FatalIfErrorf(clusterapis.AddToScheme(mgr.GetScheme()), "Cannot add Cluster Kafka APIs to scheme")
	ctx.FatalIfErrorf(namespacedapis.AddToScheme(mgr.GetScheme()), "Cannot add Namespaced Kafka APIs to scheme")
	ctx.FatalIfErrorf(apiextensionsv1.AddToScheme(mgr.GetScheme()), "Cannot add CustomResourceDefinition to scheme")

	metricRecorder := managed.NewMRMetricRecorder()
	stateMetrics := statemetrics.NewMRStateMetrics()

	metrics.Registry.MustRegister(metricRecorder)
	metrics.Registry.MustRegister(stateMetrics)

	ctx.FatalIfErrorf(err, "Cannot get provider")
	o := controller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: cli.MaxReconcileRate,
		PollInterval:            cli.PollInterval,
		GlobalRateLimiter:       ratelimiter.NewGlobal(cli.MaxReconcileRate),
		Features:                &feature.Flags{},
		MetricOptions: &controller.MetricOptions{
			PollStateMetricInterval: cli.PollStateMetricInterval,
			MRMetrics:               metricRecorder,
			MRStateMetrics:          stateMetrics,
		},
	}

	if cli.EnableManagementPolicies {
		o.Features.Enable(feature.EnableBetaManagementPolicies)
		log.Info("Beta feature enabled", "flag", feature.EnableBetaManagementPolicies)
	}

	if cli.EnableChangeLogs {
		o.Features.Enable(feature.EnableAlphaChangeLogs)
		log.Info("Alpha feature enabled", "flag", feature.EnableAlphaChangeLogs)

		conn, err := grpc.NewClient("unix://"+cli.ChangelogsSocketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
		ctx.FatalIfErrorf(err, "failed to create change logs client connection at %s", cli.ChangelogsSocketPath)

		clo := controller.ChangeLogOptions{
			ChangeLogger: managed.NewGRPCChangeLogger(
				changelogsv1alpha1.NewChangeLogServiceClient(conn),
				managed.WithProviderVersion(fmt.Sprintf("provider-kafka:%s", version.Version))),
		}
		o.ChangeLogOptions = &clo
	}

	canSafeStart, err := canWatchCRD(context.Background(), mgr)

	ctx.FatalIfErrorf(err, "SafeStart precheck failed")

	if canSafeStart {
		o.Gate = new(gate.Gate[schema.GroupVersionKind])
		ctx.FatalIfErrorf(clustercontroller.SetupGated(mgr, o), "Cannot setup Cluster Kafka controllers")
		ctx.FatalIfErrorf(namespacedcontroller.SetupGated(mgr, o), "Cannot setup Namespaced Kafka controllers")
		ctx.FatalIfErrorf(customresourcesgate.Setup(mgr, o), "Cannot setup CRD gate controller")
	} else {
		log.Info("Provider has missing RBAC permissions for watching CRDs, controller SafeStart capability will be disabled")
		ctx.FatalIfErrorf(clustercontroller.Setup(mgr, o), "Cannot setup Cluster Kafka controllers")
		ctx.FatalIfErrorf(namespacedcontroller.Setup(mgr, o), "Cannot setup Namespaced Kafka controllers")
	}

	ctx.FatalIfErrorf(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}

func canWatchCRD(ctx context.Context, mgr manager.Manager) (bool, error) {
	if err := authv1.AddToScheme(mgr.GetScheme()); err != nil {
		return false, err
	}
	verbs := []string{"get", "list", "watch"}
	for _, verb := range verbs {
		sar := &authv1.SelfSubjectAccessReview{
			Spec: authv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authv1.ResourceAttributes{
					Group:    "apiextensions.k8s.io",
					Resource: "customresourcedefinitions",
					Verb:     verb,
				},
			},
		}
		if err := mgr.GetClient().Create(ctx, sar); err != nil {
			return false, errors.Wrapf(err, "unable to perform RBAC check for verb %s on CustomResourceDefinitions", verb)
		}
		if !sar.Status.Allowed {
			return false, nil
		}
	}
	return true, nil
}
