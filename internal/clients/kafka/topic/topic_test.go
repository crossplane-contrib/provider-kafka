package topic

import (
	"fmt"
	"github.com/crossplane-contrib/provider-kafka/apis/topic/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestGenerate(t *testing.T) {
	type args struct {
		name   string
		params *v1alpha1.TopicParameters
	}

	type want struct {
		topic *Topic
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ValidComparison": {
			args: args{
				name: "validComparison",
				params: &v1alpha1.TopicParameters{
					ReplicationFactor: 1,
					Partitions:        1,
					Config:            nil,
				},
			},
			want: want{
				&Topic{
					Name:              "validComparison",
					ReplicationFactor: 1,
					Partitions:        1,
					Config:            nil,
				},
			},
		},
		"NameDifference": {
			args: args{
				name: "nameDiff",
				params: &v1alpha1.TopicParameters{
					ReplicationFactor: 1,
					Partitions:        1,
					Config:            nil,
				},
			},
			want: want{
				&Topic{
					Name:              "nameDifferent",
					ReplicationFactor: 1,
					Partitions:        1,
					Config:            nil,
				},
			},
		},
		"ReplicationFactorDifference": {
			args: args{
				name: "repFactorDiff",
				params: &v1alpha1.TopicParameters{
					ReplicationFactor: 1,
					Partitions:        1,
					Config:            nil,
				},
			},
			want: want{
				&Topic{
					Name:              "repFactorDiff",
					ReplicationFactor: 3,
					Partitions:        1,
					Config:            nil,
				},
			},
		},
		"PartitionsDifference": {
			args: args{
				name: "partitionsDiff",
				params: &v1alpha1.TopicParameters{
					ReplicationFactor: 1,
					Partitions:        3,
					Config:            nil,
				},
			},
			want: want{
				&Topic{
					Name:              "partitionsDiff",
					ReplicationFactor: 1,
					Partitions:        1,
					Config:            nil,
				},
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			topic := Generate(tt.args.name, tt.args.params)
			fmt.Println(topic)
			if diff := cmp.Diff(tt.want.topic, topic); diff != "" {
				t.Errorf("Generate() =  -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIsUpToDate(t *testing.T) {
	type args struct {
		in       *v1alpha1.TopicParameters
		observed *Topic
	}

	cases := map[string]struct {
		name string
		args args
		want bool
	}{
		"IsUpToDate": {
			name: "upToDate",
			args: args{
				in: &v1alpha1.TopicParameters{
					ReplicationFactor: 1,
					Partitions:        1,
					Config:            nil,
				},
				observed: &Topic{
					Name:              "upToDate",
					ReplicationFactor: 1,
					Partitions:        1,
					Config:            nil,
				},
			},
			want: true,
		},
		"DiffReplicationFactor": {
			name: "repFactorDiff",
			args: args{
				in: &v1alpha1.TopicParameters{
					ReplicationFactor: 2,
					Partitions:        1,
					Config:            nil,
				},
				observed: &Topic{
					Name:              "k25",
					ReplicationFactor: 1,
					Partitions:        1,
					Config:            nil,
				},
			},
			want: false,
		},
	}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			isUpToDate := IsUpToDate(tt.args.in, tt.args.observed)
			fmt.Println(isUpToDate)
			if diff := cmp.Diff(tt.want, isUpToDate); diff != "" {
				t.Errorf("IsUpToDate() = -want +got")
			}

		})
	}
}
