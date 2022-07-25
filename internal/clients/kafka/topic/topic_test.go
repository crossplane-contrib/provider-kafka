package topic

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/crossplane-contrib/provider-kafka/apis/topic/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"

	"github.com/google/go-cmp/cmp"
	"github.com/twmb/franz-go/pkg/kadm"
)

var kafkaPassword = os.Getenv("KAFKA_PASSWORD")

var dataTesting = []byte(
	fmt.Sprintf(`{
			"brokers": [ "kafka-dev-0.kafka-dev-headless:9092"],
			"sasl": {
				"mechanism": "PLAIN",
				"username": "user",
				"password": "%s"
			}
		}`, kafkaPassword),
)

func TestCreate(t *testing.T) {

	newAc, _ := kafka.NewAdminClient(context.Background(), dataTesting, nil)

	type args struct {
		ctx    context.Context
		client *kadm.Client
		topic  *Topic
	}
	{
		cases := map[string]struct {
			name    string
			args    args
			wantErr bool
		}{
			"CreateTopicOne": {
				name: "CreateTopicOne",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              "testTopic-1",
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: false,
			},

			"CreateTopicTwo": {
				name: "CreateTopicTwo",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              "testTopic-2",
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: false,
			},

			"CreateTopicThree": {
				name: "CreateTopicThree",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              "testTopic-3",
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: false,
			},
		}

		// TODO: Add test cases.

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				if err := Create(tt.args.ctx, tt.args.client, tt.args.topic); (err != nil) != tt.wantErr {
					t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	}
}

func TestGet(t *testing.T) {

	newAc, _ := kafka.NewAdminClient(context.Background(), dataTesting, nil)

	type args struct {
		ctx    context.Context
		client *kadm.Client
		name   string
	}
	cases := map[string]struct {
		name    string
		args    args
		want    *Topic
		wantErr bool
	}{
		"GetTopicWorked": {
			name: "GetTopicWorked",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   "testTopic-1",
			},
			want: &Topic{
				Name:              "testTopic-1",
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            nil,
			},
			wantErr: false,
		},
		"GetTopicDoesNotExist": {
			name: "GetTopicDoesNotExist",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   "testTopic-00",
			},
			want: &Topic{
				Name:              "testTopic-1",
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Get(tt.args.ctx, tt.args.client, tt.args.name)
			fmt.Println(err)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

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

func TestCreateDuplicateTopic(t *testing.T) {

	newAc, _ := kafka.NewAdminClient(context.Background(), dataTesting, nil)

	fmt.Printf("------Checking duplicate topic creation logic------")

	type args struct {
		ctx    context.Context
		client *kadm.Client
		topic  *Topic
	}
	{
		cases := map[string]struct {
			name    string
			args    args
			wantErr bool
		}{
			"CreateTopicOneExists": {
				name: "CreateTopicOneExists",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              "testTopic-1",
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: true,
			},

			"CreateTopicTwoExists": {
				name: "CreateTopicTwoExists",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              "testTopic-2",
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: true,
			},

			"CreateTopicThreeExists": {
				name: "CreateTopicThreeExists",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              "testTopic-3",
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: true,
			},
			"CreateTopicDoesNotExist": {
				name: "CreateTopicDoesNotExist",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              "createTopic-doesNot-exist",
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: false,
			},
		}

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				if err := Create(tt.args.ctx, tt.args.client, tt.args.topic); (err != nil) != tt.wantErr {
					t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	}
}

func TestDelete(t *testing.T) {

	newAc, _ := kafka.NewAdminClient(context.Background(), dataTesting, nil)

	type args struct {
		ctx    context.Context
		client *kadm.Client
		name   string
	}
	cases := map[string]struct {
		name    string
		args    args
		wantErr bool
	}{
		"DeleteTopicOne": {
			name: "DeleteTopicOne",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   "testTopic-1",
			},
			wantErr: false,
		},
		"DeleteTopicTwo": {
			name: "DeleteTopicTwo",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   "testTopic-2",
			},
			wantErr: false,
		},
		"DeleteTopicThree": {
			name: "DeleteTopicThree",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   "testTopic-3",
			},
			wantErr: false,
		},
		"DeleteTopicThreeDoesNotExist": {
			name: "DeleteTopicThreeDoesNotExist",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   "create3",
			},
			wantErr: true,
		},
		"DeleteTopicThatDoesNotExist": {
			name: "DeleteTopicThatDoesNotExist",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   "createTopic-doesNot-exist",
			},
			wantErr: false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := Delete(tt.args.ctx, tt.args.client, tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
