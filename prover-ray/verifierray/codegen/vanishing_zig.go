package codegen

import (
	"fmt"
	"io"
	"strings"
)

// VanishingZigOptions configures imports and wrapping sections for generated
// Zig vanishing.System data.
type VanishingZigOptions struct {
	FieldImport     string
	VanishingImport string
	EmitHeader      bool
	EmitSystemsList bool
}

func defaultVanishingZigOptions() VanishingZigOptions {
	return VanishingZigOptions{
		FieldImport:     `@import("../field/koalabear.zig")`,
		VanishingImport: `@import("../query/vanishing.zig")`,
		EmitHeader:      true,
		EmitSystemsList: true,
	}
}

// WriteVanishingScenariosZig writes generated Zig source that contains the
// vanishing.System values for the supplied cases. It emits data only; Zig owns
// the evaluator and quotient identity implementation.
func WriteVanishingScenariosZig(w io.Writer, cases []VanishingSystem) error {
	return WriteVanishingScenariosZigWithOptions(w, cases, defaultVanishingZigOptions())
}

func WriteVanishingScenariosZigWithOptions(w io.Writer, cases []VanishingSystem, opts VanishingZigOptions) error {
	data := vanishingTemplateData{
		Options: opts,
		Cases:   make([]vanishingTemplateCase, len(cases)),
	}
	for i, tc := range cases {
		data.Cases[i] = newVanishingTemplateCase(i, tc)
	}
	return executeVanishingTemplate(w, data)
}

func WriteVanishingSystemZig(w io.Writer, index int, system VanishingSystem) error {
	return WriteVanishingSystemZigWithOptions(w, index, system, defaultVanishingZigOptions())
}

func WriteVanishingSystemZigWithOptions(
	w io.Writer, index int, system VanishingSystem, opts VanishingZigOptions,
) error {
	data := vanishingTemplateData{
		Options: opts,
		Cases:   []vanishingTemplateCase{newVanishingTemplateCase(index, system)},
	}
	return executeVanishingTemplate(w, data)
}

func executeVanishingTemplate(w io.Writer, data vanishingTemplateData) error {
	return executeZigTemplate(w, "vanishing", data)
}

type vanishingTemplateData struct {
	Options VanishingZigOptions
	Cases   []vanishingTemplateCase
}

type vanishingTemplateCase struct {
	Index   int
	Name    string
	System  VanishingSystem
	Modules []vanishingTemplateModule
}

type vanishingTemplateModule struct {
	Index   int
	Module  VanishingModule
	Buckets []vanishingTemplateBucket
}

func (m vanishingTemplateModule) SingleExpressionName() string {
	if len(m.Buckets) != 1 || len(m.Buckets[0].Bucket.Vanishings) != 1 {
		return ""
	}
	return m.Buckets[0].Bucket.Vanishings[0].SourceName
}

type vanishingTemplateBucket struct {
	ModuleIndex int
	Index       int
	Bucket      VanishingBucket
}

func newVanishingTemplateCase(index int, tc VanishingSystem) vanishingTemplateCase {
	out := vanishingTemplateCase{
		Index:   index,
		Name:    tc.SourceName,
		System:  tc,
		Modules: make([]vanishingTemplateModule, len(tc.Modules)),
	}
	for moduleIndex, module := range tc.Modules {
		moduleView := vanishingTemplateModule{
			Index:   moduleIndex,
			Module:  module,
			Buckets: make([]vanishingTemplateBucket, len(module.Buckets)),
		}
		for bucketIndex, bucket := range module.Buckets {
			moduleView.Buckets[bucketIndex] = vanishingTemplateBucket{
				ModuleIndex: moduleIndex,
				Index:       bucketIndex,
				Bucket:      bucket,
			}
		}
		out.Modules[moduleIndex] = moduleView
	}
	return out
}

// indexMax is the largest value vanishing.Index (u16) holds. Every index the
// generated System stores — expression positions, claim offsets, coin indices —
// must fit, or the verifier would silently read a truncated value.
const indexMax = 1<<16 - 1

// checkIndex panics with the offending value rather than emitting a literal the
// verifier cannot represent.
func checkIndex(v int, what string) int {
	if v < 0 || v > indexMax {
		panic(fmt.Sprintf("%s is %d; vanishing.Index (u16) holds 0..%d", what, v, indexMax))
	}
	return v
}

func exprNodeLiteral(expr ExprNode) string {
	switch expr.Kind {
	case ExprColumnClaim:
		return fmt.Sprintf(".{ .column_claim = %d }, // col: \"%s\"",
			checkIndex(expr.ColumnClaim, "column claim"), ZigString(expr.ColumnSourceName))
	case ExprCellValue:
		return fmt.Sprintf(".{ .cell_value = .{ .round = %d, .index = %d } }, // cell: \"%s\"",
			checkIndex(expr.Cell.Round, "cell round"), checkIndex(expr.Cell.Index, "cell index"),
			ZigString(expr.Cell.SourceName))
	case ExprCoinValue:
		return fmt.Sprintf(".{ .coin_value = %d }, // coin: \"%s\"",
			checkIndex(expr.Coin.FlatIndex, "coin index"), ZigString(expr.Coin.SourceName))
	case ExprConstant:
		return fmt.Sprintf(".{ .constant = .{ .value = %d } },", expr.Constant.Uint64())
	case ExprOp:
		// Unary operators carry only lhs; rhs is a don't-care the verifier
		// never reads (see vanishing.zig's ExprOp). Reject anything wider
		// rather than silently dropping operands into a verifier that cannot
		// represent them.
		if len(expr.Operands) == 0 || len(expr.Operands) > 2 {
			panic(fmt.Sprintf("expression operator %q has %d operands; only 1 or 2 are supported", expr.Operator, len(expr.Operands)))
		}
		rhs := 0
		if len(expr.Operands) == 2 {
			rhs = expr.Operands[1]
		}
		return fmt.Sprintf(".{ .op = .{ .operator = .%s, .lhs = %d, .rhs = %d } },",
			expr.Operator, checkIndex(expr.Operands[0], "operand lhs"), checkIndex(rhs, "operand rhs"))
	case ExprLagrangeSelector:
		return fmt.Sprintf(".{ .lagrange_selector = %d },", expr.SelectorPosition)
	default:
		panic(fmt.Sprintf("unknown ExprKind %d", int(expr.Kind)))
	}
}

func moduleSizeLiteral(size ModuleSize) string {
	if size.Dynamic {
		return fmt.Sprintf(".{ .dynamic = %d }", size.DynamicIndex)
	}
	return fmt.Sprintf(".{ .static = %d }", size.StaticSize)
}

func IntSlice(values []int) string {
	return "." + intArray(values)
}

func intArray(values []int) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprintf("%d", value)
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func ZigString(value string) string {
	return strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(value)
}
