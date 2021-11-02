module github.com/crossplane-contrib/provider-kafka

go 1.16

require (
	github.com/crossplane/crossplane-runtime v0.15.0
	github.com/crossplane/crossplane-tools v0.0.0-20210320162312-1baca298c527
	github.com/google/go-cmp v0.5.6
	github.com/pkg/errors v0.9.1
	github.com/twmb/franz-go v1.2.2
	github.com/twmb/franz-go/pkg/kadm v0.0.0-20211021211700-b415080bed36
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	sigs.k8s.io/controller-runtime v0.9.6
	sigs.k8s.io/controller-tools v0.6.2
)
