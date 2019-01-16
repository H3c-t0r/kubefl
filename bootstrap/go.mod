module github.com/kubeflow/kubeflow/bootstrap

require (
	cloud.google.com/go v0.32.0
	contrib.go.opencensus.io/exporter/stackdriver v0.0.0-20180910204836-9f333b48d382
	github.com/Azure/go-ansiterm v0.0.0-20170629204627-19f72df4d05d
	github.com/Azure/go-autorest v9.9.0+incompatible
	github.com/BurntSushi/toml v0.3.1
	github.com/GeertJohan/go.rice v0.0.0-20170420135705-c02ca9a983da
	github.com/MakeNowJust/heredoc v0.0.0-20170808103936-bb23615498cd
	github.com/Masterminds/semver v1.3.1
	github.com/Masterminds/sprig v2.16.0+incompatible
	github.com/PuerkitoBio/purell v1.0.0
	github.com/PuerkitoBio/urlesc v0.0.0-20160726150825-5bd2802263f2
	github.com/aokoli/goutils v1.0.1
	github.com/asaskevich/govalidator v0.0.0-20160715170612-593d64559f76
	github.com/beorn7/perks v0.0.0-20180321164747-3a771d992973
	github.com/blang/semver v3.5.0+incompatible
	github.com/cenkalti/backoff v2.0.0+incompatible
	github.com/daaku/go.zipexe v0.0.0-20150329023125-a5fe2436ffcb
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v0.0.0-20160705203006-01aeca54ebda
	github.com/docker/distribution v0.0.0-20170726174610-edc3ab29cdff
	github.com/docker/docker v0.0.0-20170731201938-4f3616fb1c11
	github.com/docker/go-connections v0.3.0
	github.com/docker/go-units v0.0.0-20170127094116-9e638d38cf69
	github.com/docker/spdystream v0.0.0-20160310174837-449fdfce4d96
	github.com/emicklei/go-restful v0.0.0-20170410110728-ff4f55a20633
	github.com/emicklei/go-restful-swagger12 v0.0.0-20170926063155-7524189396c6
	github.com/evanphx/json-patch v0.0.0-20170719203123-944e07253867
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d
	github.com/fatih/camelcase v0.0.0-20160318181535-f6a740d52f96
	github.com/fatih/color v0.0.0-20180516100307-2d684516a886
	github.com/ghodss/yaml v1.0.0
	github.com/go-kit/kit v0.7.0
	github.com/go-logfmt/logfmt v0.3.0
	github.com/go-logr/logr v0.1.0 // indirect
	github.com/go-logr/zapr v0.1.0 // indirect
	github.com/go-openapi/analysis v0.0.0-20160815203709-b44dc874b601
	github.com/go-openapi/errors v0.0.0-20160704190347-d24ebc2075ba
	github.com/go-openapi/jsonpointer v0.0.0-20160704185906-46af16f9f7b1
	github.com/go-openapi/jsonreference v0.0.0-20160704190145-13c6e3589ad9
	github.com/go-openapi/loads v0.0.0-20170520182102-a80dea3052f0
	github.com/go-openapi/runtime v0.0.0-20160704190703-11e322eeecc1
	github.com/go-openapi/spec v0.0.0-20180213232550-1de3e0542de6
	github.com/go-openapi/strfmt v0.0.0-20160812050534-d65c7fdb29ec
	github.com/go-openapi/swag v0.0.0-20170606142751-f3f9494671f9
	github.com/go-openapi/validate v0.0.0-20171117174350-d509235108fc
	github.com/go-stack/stack v1.8.0
	github.com/gobwas/glob v0.2.3
	github.com/gogo/protobuf v0.0.0-20170330071051-c0656edd0d9e
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20160516000752-02826c3e7903
	github.com/golang/protobuf v1.2.0
	github.com/google/btree v0.0.0-20160524151835-7d79101e329e
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-jsonnet v0.11.2
	github.com/google/go-querystring v0.0.0-20170111101155-53e6ce116135
	github.com/google/gofuzz v0.0.0-20161122191042-44d81051d367
	github.com/google/uuid v0.0.0-20171113160352-8c31c18f31ed
	github.com/googleapis/gax-go v2.0.0+incompatible
	github.com/googleapis/gnostic v0.0.0-20170729233727-0c5108395e2d
	github.com/gophercloud/gophercloud v0.0.0-20180210024343-6da026c32e2d
	github.com/gregjones/httpcache v0.0.0-20170728041850-787624de3eb7
	github.com/hashicorp/golang-lru v0.0.0-20160207214719-a0d98a5f2880
	github.com/howeyc/gopass v0.0.0-20170109162249-bf9dde6d0d2c
	github.com/huandu/xstrings v0.0.0-20180906151751-8bbcf2f9ccb5
	github.com/imdario/mergo v0.0.0-20141206190957-6633656539c1
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/jonboulle/clockwork v0.0.0-20180716110948-e7c6d408fd5c
	github.com/json-iterator/go v0.0.0-20171212105241-13f86432b882
	github.com/kardianos/osext v0.0.0-20150410034420-8fef92e41e22
	github.com/konsorten/go-windows-terminal-sequences v1.0.1
	github.com/kr/fs v0.1.0
	github.com/kr/logfmt v0.0.0-20140226030751-b84e30acd515
	github.com/ksonnet/ksonnet v0.13.1
	github.com/ksonnet/ksonnet-lib v0.1.12
	github.com/mailru/easyjson v0.0.0-20170624190925-2f5df55504eb
	github.com/mattn/go-colorable v0.0.0-20180310133214-efa589957cd0
	github.com/mattn/go-isatty v0.0.0-20180830101745-3fb116b82035
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/mitchellh/go-homedir v1.0.0 // indirect
	github.com/mitchellh/go-wordwrap v0.0.0-20150314170334-ad45545899c7
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/onrik/logrus v0.0.0-20180801161715-ca0a758702be
	github.com/onsi/ginkgo v1.7.0 // indirect
	github.com/onsi/gomega v1.4.3
	github.com/opencontainers/go-digest v0.0.0-20170106003457-a6d0ee40d420
	github.com/opencontainers/image-spec v0.0.0-20170604055404-372ad780f634
	github.com/pborman/uuid v0.0.0-20150603214016-ca53cad383ca
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/pkg/errors v0.8.0
	github.com/pkg/sftp v0.0.0-20180824225003-b9345f483dc3
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v0.8.0
	github.com/prometheus/client_model v0.0.0-20180712105110-5c3871d89910
	github.com/prometheus/common v0.0.0-20180801064454-c7de2306084e
	github.com/prometheus/procfs v0.0.0-20180725123919-05ee40e3a273
	github.com/russross/blackfriday v0.0.0-20151117072312-300106c228d5
	github.com/shazow/go-diff v0.0.0-20160112020656-b6b7b6733b8c
	github.com/shurcooL/sanitized_anchor_name v0.0.0-20151028001915-10ef21a441db
	github.com/sirupsen/logrus v1.2.0
	github.com/spf13/afero v1.1.2
	github.com/spf13/cobra v0.0.0-20180228053838-6644d46b81fa
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.3.1 // indirect
	github.com/stretchr/testify v1.2.2
	go.opencensus.io v0.15.0
	go.uber.org/atomic v1.3.2 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.9.1 // indirect
	golang.org/x/crypto v0.0.0-20181203042331-505ab145d0a9
	golang.org/x/net v0.0.0-20180906233101-161cd47e91fd
	golang.org/x/oauth2 v0.0.0-20180821212333-d2e6202438be
	golang.org/x/sys v0.0.0-20181205085412-a5c9d58dba9a
	golang.org/x/text v0.3.0
	golang.org/x/time v0.0.0-20161028155119-f51c12702a4d
	golang.org/x/vgo v0.0.0-20180912184537-9d567625acf4 // indirect
	google.golang.org/api v0.0.0-20180910000450-7ca32eb868bf
	google.golang.org/appengine v1.1.0
	google.golang.org/genproto v0.0.0-20180911211118-36d5787dc535
	google.golang.org/grpc v1.16.0
	gopkg.in/inf.v0 v0.9.0
	gopkg.in/square/go-jose.v2 v2.1.3
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20181221193117-173ce66c1e39
	k8s.io/apiextensions-apiserver v0.0.0-20180908152229-2c1d23e4c7d6
	k8s.io/apimachinery v0.0.0-20190111195121-fa6ddc151d63
	k8s.io/apiserver v0.0.0-20180426121757-0841753fc26e
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/helm v0.0.0-20180910190057-b4b693c31684
	k8s.io/klog v0.1.0 // indirect
	k8s.io/kube-openapi v0.0.0-20180216212618-50ae88d24ede
	k8s.io/kubernetes v1.10.2
	k8s.io/utils v0.0.0-20171122000934-aedf551cdb8b
	sigs.k8s.io/controller-runtime v0.1.9
	sigs.k8s.io/testing_frameworks v0.1.1 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
	vbom.ml/util v0.0.0-20160121211510-db5cfe13f5cc
)
