package outputcustomnames

import (
	"github.com/remiges-tech/alya/di"
	"github.com/remiges-tech/alya/cmd/alya-di/testdata/output_custom_names/svc"
)

var Graph = di.New(
	di.Provide(newFooHandler, newBarHandler, newDefaultSvc),
	di.Outputs(
		di.Named("PrimaryHandler", di.Type[*svc.Foo]()),
		di.Named("SecondaryHandler", di.Type[*svc.Bar]()),
		di.Type[*svc.DefaultSvc](),
	),
)

func newFooHandler() *svc.Foo {
	return &svc.Foo{}
}

func newBarHandler() *svc.Bar {
	return &svc.Bar{}
}

func newDefaultSvc() *svc.DefaultSvc {
	return &svc.DefaultSvc{}
}
