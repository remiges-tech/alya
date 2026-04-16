package outputfieldnames

import (
	"github.com/remiges-tech/alya/di"
	"github.com/remiges-tech/alya/cmd/alya-di/testdata/output_field_names/bar"
	"github.com/remiges-tech/alya/cmd/alya-di/testdata/output_field_names/foo"
)

var Graph = di.New(
	di.Provide(newFooHandler, newBarHandler),
	di.Outputs(
		di.Type[*foo.Handler](),
		di.Type[*bar.Handler](),
	),
)

func newFooHandler() *foo.Handler {
	return &foo.Handler{}
}

func newBarHandler() *bar.Handler {
	return &bar.Handler{}
}
