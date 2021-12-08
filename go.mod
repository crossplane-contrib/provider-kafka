module github.com/crossplane-contrib/provider-kafka

go 1.16

require (
	github.com/confluentinc/confluent-kafka-go v1.7.0
	github.com/crossplane/crossplane-runtime v0.15.0
	github.com/crossplane/crossplane-tools v0.0.0-20210320162312-1baca298c527
	github.com/pkg/errors v0.9.1
	github.com/twmb/franz-go v1.2.3-0.20211102021212-9a7f9860bbb6 // indirect
	github.com/twmb/franz-go/pkg/kadm v0.0.0-20211102021212-9a7f9860bbb6
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	sigs.k8s.io/controller-runtime v0.9.6
	sigs.k8s.io/controller-tools v0.6.2
)
