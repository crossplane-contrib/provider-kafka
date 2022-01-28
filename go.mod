module github.com/crossplane-contrib/provider-kafka

go 1.16

require (
	github.com/crossplane/crossplane-runtime v0.15.0
	github.com/crossplane/crossplane-tools v0.0.0-20210320162312-1baca298c527
	github.com/google/go-cmp v0.5.6
	github.com/pkg/errors v0.9.1
	github.com/twmb/franz-go v1.2.3
	github.com/twmb/franz-go/pkg/kadm v0.0.0-20211102021212-9a7f9860bbb6
	github.com/twmb/franz-go/pkg/kmsg v0.0.0-20211104051938-70808186d5f7
	github.com/twmb/kcl v0.8.0 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	sigs.k8s.io/controller-runtime v0.9.6
	sigs.k8s.io/controller-tools v0.6.2
)
