package common

import (
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
)

// fakeGate records registrations and runs callbacks on demand.
type fakeGate struct {
	callback func()
	gvks     []schema.GroupVersionKind
}

func (g *fakeGate) Register(cb func(), gvks ...schema.GroupVersionKind) {
	g.callback, g.gvks = cb, gvks
}

func (g *fakeGate) Set(schema.GroupVersionKind, bool) bool { return false }

func TestSetupGated(t *testing.T) {
	t.Parallel()

	gvk := schema.GroupVersionKind{Group: "kafka.crossplane.io", Version: "v1alpha1", Kind: "Connector"}

	gate := &fakeGate{}
	called := 0
	require.NoError(t, SetupGated(nil, controller.Options{Gate: gate}, gvk, func(ctrl.Manager, controller.Options) error {
		called++
		return nil
	}))
	assert.Equal(t, []schema.GroupVersionKind{gvk}, gate.gvks)
	assert.Zero(t, called, "setup must wait for the gate")

	gate.callback()
	assert.Equal(t, 1, called)

	failing := &fakeGate{}
	require.NoError(t, SetupGated(nil, controller.Options{Gate: failing}, gvk, func(ctrl.Manager, controller.Options) error {
		return errors.New("boom")
	}))
	assert.PanicsWithError(t, "cannot setup Connector controller: boom", failing.callback)
}
