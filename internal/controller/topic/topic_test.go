package topic

import (
	"context"
	"reflect"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/twmb/franz-go/pkg/kadm"
)

func Test_external_Observe(t *testing.T) {
	type fields struct {
		kafkaClient *kadm.Client
		log         logging.Logger
	}
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	tests := map[string]struct {
		name    string
		fields  fields
		args    args
		want    managed.ExternalObservation
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &external{
				kafkaClient: tt.fields.kafkaClient,
				log:         tt.fields.log,
			}
			got, err := c.Observe(tt.args.ctx, tt.args.mg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Observe() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Observe() got = %v, want %v", got, tt.want)
			}
		})
	}
}
