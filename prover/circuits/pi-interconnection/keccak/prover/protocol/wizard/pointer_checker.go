package wizard

import (
	"reflect"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// checkAnyInStore recursively crawls in the fields of the provided empty-interface
// object. It relies on the reflect package to crawl in the internal struct
// fields, arrays, slices or maps. It sanity-checks that all the contained
// column and expression objects are linked to the store of the compiled IOP.
//
// This is useful to early-catch bugs that would otherwise hard to find.
func (comp *CompiledIOP) checkAnyInStore(x any) {

	if !comp.WithStorePointerChecks {
		return
	}

	comp.checkReflectValueInStore(reflect.ValueOf(x))
}

// checkReflectValueInStore recursively crawls in the fields of the provided
// reflect.Value object.
func (comp *CompiledIOP) checkReflectValueInStore(x reflect.Value) {

	if !comp.WithStorePointerChecks {
		return
	}

	if !x.IsValid() {
		return
	}

	colType := reflect.TypeOf((*ifaces.Column)(nil)).Elem()
	exprType := reflect.TypeOf((*symbolic.Expression)(nil))

	switch x.Type() {
	case colType:
		comp.checkColumnInStore(x.Interface().(ifaces.Column))
	case exprType:
		comp.checkExpressionInStore(x.Interface().(*symbolic.Expression))
	}

	switch x.Kind() {
	case reflect.Interface:
		comp.checkReflectValueInStore(x.Elem())
	case reflect.Ptr:
		comp.checkReflectValueInStore(x.Elem())
	case reflect.Struct:
		for i := 0; i < x.NumField(); i++ {
			comp.checkReflectValueInStore(x.Field(i))
		}
	case reflect.Array:
		for i := 0; i < x.Len(); i++ {
			comp.checkReflectValueInStore(x.Index(i))
		}
	case reflect.Slice:
		for i := 0; i < x.Len(); i++ {
			comp.checkReflectValueInStore(x.Index(i))
		}
	case reflect.Map:
		for _, key := range x.MapKeys() {
			comp.checkReflectValueInStore(x.MapIndex(key))
		}
	}
}

// checkColumnInStore checks that the provided column is in the store of the
// compiled IOP. It is used to early-catch bugs that would otherwise hard to
// find.
func (comp *CompiledIOP) checkColumnInStore(col ifaces.Column) {

	if !comp.WithStorePointerChecks {
		return
	}

	if nat, ok := col.(column.Natural); ok {
		if nat.GetStoreUnsafe() != comp.Columns {
			utils.Panic("column %v has a wrong store", col.GetColID())
		}
	}
}

// checkExpressionInStore checks that the provided expression is in the store of
// the compiled IOP. It is used to early-catch bugs that would otherwise hard to
// find.
func (comp *CompiledIOP) checkExpressionInStore(expr *symbolic.Expression) {

	if !comp.WithStorePointerChecks {
		return
	}

	meta := expr.BoardListVariableMetadata()

	for _, m := range meta {
		if col, ok := m.(ifaces.Column); ok {
			comp.checkColumnInStore(col)
		}
	}
}
