// Package csvtraces provides a way to read and write traces in CSV format.
package csvtraces

import (
	"encoding/csv"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/protocol/query"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

type cfg struct {
	// The number of rows in the trace
	nbRows             int
	skipPrePaddingZero bool
	filterOn           ifaces.Column
	inHex              bool
	renameCols         []string
}

type Option func(*cfg) error

// Octuplet represents an octuplet of columns that can be printed in a frienly way.
type Octuplet struct {
	V    [8]ifaces.Column
	Name string
}

// WithNbRows sets the number of rows in the trace
func WithNbRows(nbRows int) Option {
	return func(c *cfg) error {
		c.nbRows = nbRows
		return nil
	}
}

// SkipPrepaddingZero skips the zeroes at the beginning of the file
func SkipPrepaddingZero(c *cfg) error {
	c.skipPrePaddingZero = true
	return nil
}

// FilterOn sets the CSV printer to ignore rows where the provided filter
// column is zero.
func FilterOn(col ifaces.Column) Option {
	return func(c *cfg) error {
		c.filterOn = col
		return nil
	}
}

// InHex sets the CSV printer to print the values in hexadecimal
func InHex(c *cfg) error {
	c.inHex = true
	return nil
}

// RenameCols rename the columns in the csv
func RenameCols(s ...string) Option {
	return func(c *cfg) error {
		c.renameCols = s
		return nil
	}
}

type CsvTrace struct {
	mapped map[string][]*big.Int
	nbRows int
}

func MustOpenCsvFile(fName string) *CsvTrace {

	f, err := os.Open(fName)
	if err != nil {
		utils.Panic("%v", err.Error())
	}
	defer f.Close()

	ct, err := NewCsvTrace(f)
	if err != nil {
		utils.Panic("could not parse CSV: %v", err.Error())
	}

	return ct
}

// FmtCsv is a utility function that can be used in order to print a set of column
// in a csv format so that debugging and testcase generation are simpler. It can
// take either plain [ifaces.Column] objects as input or limbs.Limbed objects.
func FmtCsv(w io.Writer, run ifaces.Runtime, objs []any, options []Option) error {

	var (
		header       = []string{}
		assignment   = [][]*big.Int{}
		cfg          = cfg{}
		foundNonZero = false
		filterCol    []field.Element
	)

	for _, op := range options {
		op(&cfg)
	}

	if cfg.renameCols != nil && len(cfg.renameCols) != len(objs) {
		utils.Panic("provided %v columns, but also provided %v name replacements", len(objs), len(cfg.renameCols))
	}

	for i, obj := range objs {

		switch obj := obj.(type) {

		case Octuplet:
			name := obj.Name
			if cfg.renameCols != nil {
				name = cfg.renameCols[i]
			}
			header = append(header, name)

			vs := [8][]field.Element{}
			for j := range vs {
				vs[j] = obj.V[j].GetColAssignment(run).IntoRegVecSaveAlloc()
			}

			a := make([]*big.Int, 0)
			for i := range vs[0] {
				f := big.NewInt(0)
				for j := range vs {
					var s big.Int
					vs[j][i].BigInt(&s)
					f.Lsh(f, 32)
					f.Add(f, &s)
				}
				a = append(a, f)
			}

			assignment = append(assignment, a)

		case ifaces.Column:
			name := obj.String()
			if cfg.renameCols != nil {
				name = cfg.renameCols[i]
			}
			header = append(header, name)
			a := obj.GetColAssignment(run).IntoRegVecSaveAlloc()
			assignment = append(assignment, koalaVecToBigInt(a))

		case limbs.Limbed:
			header = append(header, obj.String())
			a := obj.ToBigEndianLimbs().GetAssignmentAsBigInt(run)
			assignment = append(assignment, a)

		default:
			utils.Panic("unsupported type %T(%++v)", obj, obj)
		}
	}

	fmt.Fprintf(w, "%v\n", strings.Join(header, ","))

	if cfg.filterOn != nil {
		filterCol = cfg.filterOn.GetColAssignment(run).IntoRegVecSaveAlloc()
	}

	for r := range assignment[0] {

		var (
			fmtVals   = []string{}
			allZeroes = true
		)

		for c := range assignment {

			if !isZeroBigInt(assignment[c][r]) {
				allZeroes = false
			}

			fmtVals = append(fmtVals, fmtBigInt(cfg.inHex, assignment[c][r]))
		}

		if !allZeroes {
			foundNonZero = true
		}

		if filterCol != nil && filterCol[r].IsZero() {
			continue
		}

		if !cfg.skipPrePaddingZero || !allZeroes || foundNonZero {
			fmt.Fprintf(w, "%v\n", strings.Join(fmtVals, ","))
		}

		if cfg.nbRows > 0 && r >= cfg.nbRows {
			break
		}
	}

	return nil
}

func NewCsvTrace(r io.Reader, opts ...Option) (*CsvTrace, error) {
	cfg := &cfg{}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	rr := csv.NewReader(r)
	rr.FieldsPerRecord = 0

	data := make(map[string][]*big.Int)
	header, err := rr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header row: %w", err)
	}
	for _, h := range header {
		data[h] = make([]*big.Int, 0)
	}
	var nbRows int
	for row, err := rr.Read(); err != io.EOF; row, err = rr.Read() {
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		for i, h := range header {
			d := new(big.Int)
			if _, ok := d.SetString(row[i], 0); !ok {
				return nil, fmt.Errorf("could not decode hex string: row=%v, header=%v string=%v", i, h, row[i])
			}
			data[h] = append(data[h], d)
		}
		nbRows++
	}
	if cfg.nbRows != 0 {
		if cfg.nbRows < nbRows {
			return nil, fmt.Errorf("invalid number of rows: %d", cfg.nbRows)
		}
		nbRows = cfg.nbRows
	}

	return &CsvTrace{mapped: data, nbRows: nbRows}, nil
}

func (c *CsvTrace) GetKoala(name string) []field.Element {
	val, ok := c.mapped[name]
	if !ok {
		utils.Panic("column not found %s", name)
	}
	vKoa, err := bigIntsToKoalaStrict(val)
	if err != nil {
		utils.Panic("could not convert assignment for %s into koala, %v", name, err)
	}
	return vKoa
}

func (c *CsvTrace) GetBigInt(name string) []*big.Int {
	val, ok := c.mapped[name]
	if !ok {
		utils.Panic("csv column not found %s", name)
	}
	return val
}

// GetCommit returns a new column mapped to the current csv trace.
func (c *CsvTrace) GetCommit(b *wizard.Builder, name string) ifaces.Column {
	if _, ok := c.mapped[name]; !ok {
		utils.Panic("column not found %s", name)
	}
	length := utils.NextPowerOfTwo(c.nbRows)
	col := b.RegisterCommit(ifaces.ColID(name), length)
	return col
}

// GetLimbLe a little-endian limb object
func (c *CsvTrace) GetLimbsLe(b *wizard.Builder, name string, numLimbs int) limbs.Limbs[limbs.LittleEndian] {
	return getLimbs[limbs.LittleEndian](c, b, name, numLimbs)
}

// GetLimbBe a big-endian limb object
func (c *CsvTrace) GetLimbsBe(b *wizard.Builder, name string, numLimbs int) limbs.Limbs[limbs.BigEndian] {
	return getLimbs[limbs.BigEndian](c, b, name, numLimbs)
}

// getLimbs returns a limbs object mapped to the provided csv trace.
func getLimbs[E limbs.Endianness](c *CsvTrace, b *wizard.Builder, name string, numLimbs int) limbs.Limbs[E] {
	if _, ok := c.mapped[name]; !ok {
		utils.Panic("column not found %s", name)
	}
	length := utils.NextPowerOfTwo(c.nbRows)
	return limbs.NewLimbs[E](b.CompiledIOP, ifaces.ColID(name), numLimbs, length)

}

// AssignCols assigns a vector of columns
func (c *CsvTrace) AssignCols(run *wizard.ProverRuntime, cols ...ifaces.Column) *CsvTrace {
	for _, col := range cols {
		c.Assign(run, col)
	}
	return c
}

// Assign may assign either a column or a limb. It will panic if provided any
// other type. The function also returns a point to itself to make it chainable.
func (c *CsvTrace) Assign(run *wizard.ProverRuntime, toAssign ...any) *CsvTrace {

	for _, obj := range toAssign {

		if obj, ok := obj.(ifaces.Column); ok {

			name := string(obj.GetColID())
			vBi, ok := c.mapped[name]
			if !ok {
				utils.Panic("column not found %s", name)
			}

			vKoa, err := bigIntsToKoalaStrict(vBi)
			if err != nil {
				utils.Panic("could not convert column assignment for %v into koala, %v", name, err)
			}
			run.AssignColumn(
				obj.GetColID(), smartvectors.RightZeroPadded(vKoa, obj.Size()))

			continue
		}

		if objA, ok := obj.(limbs.Limbed); ok {
			obj := objA.ToBigEndianLimbs()
			name := obj.String()
			vBi, ok := c.mapped[name]
			if !ok {
				utils.Panic("limb not found %s", name)
			}
			obj.AssignAndZeroPadsBigInts(run, vBi)
			continue
		}

		utils.Panic("invalid type %T(%++v)", obj, obj)
	}

	return c
}

// AssignLimbsBE assigns a limb object and returns the receiver
func (c *CsvTrace) AssignLimbsBE(run *wizard.ProverRuntime, name ifaces.ColID, column []ifaces.Column) *CsvTrace {
	l := limbs.NewLimbsFromRawUnsafe[limbs.BigEndian](name, column)
	return c.Assign(run, l)
}

// AssignLimbsLE assigns a limb object and returns the receiver
func (c *CsvTrace) AssignLimbsLE(run *wizard.ProverRuntime, name ifaces.ColID, column []ifaces.Column) *CsvTrace {
	l := limbs.NewLimbsFromRawUnsafe[limbs.LittleEndian](name, column)
	return c.Assign(run, l)
}

func (c *CsvTrace) CheckAssignment(run *wizard.ProverRuntime, objects ...any) *CsvTrace {
	for _, obj := range objects {
		c.checkAssignment(run, obj)
	}
	return c
}

// CheckAssignmentCols is the same as [CheckAssignment] but specifically when
// the input is a list of columns. This allows using slice to variadic implicit
// conversion.
func (c *CsvTrace) CheckAssignmentCols(run *wizard.ProverRuntime, objects ...ifaces.Column) *CsvTrace {
	for _, obj := range objects {
		c.checkAssignment(run, obj)
	}
	return c
}

func (c *CsvTrace) checkAssignment(run *wizard.ProverRuntime, obj any) {

	var (
		wizBi []*big.Int
		csvBi []*big.Int
		name  string
		ok    bool
	)

	switch obj := obj.(type) {
	case ifaces.Column:
		name = string(obj.GetColID())
		vKoala := run.GetColumn(obj.GetColID()).IntoRegVecSaveAlloc()
		csvBi, ok = c.mapped[name]
		if !ok {
			utils.Panic("column not found in csv, %s, %v", name, utils.StringKeysOfMap(c.mapped))
		}
		wizBi = koalaVecToBigInt(vKoala)

	case limbs.Limbed:
		name = obj.String()
		wizBi = obj.ToBigEndianLimbs().GetAssignmentAsBigInt(run)
		csvBi, ok = c.mapped[name]
		if !ok {
			utils.Panic("limb not found in csv, %s", name)
		}

	default:
		utils.Panic("invalid type %T(%++v)", obj, obj)
	}

	if len(wizBi) < c.nbRows {
		utils.Panic("assignment for %s has not been assigned with the expected length, found %v in CSV and %v in wizard", name, c.nbRows, len(wizBi))
	}

	for i := 0; i < c.nbRows; i++ {
		if wizBi[i].Cmp(csvBi[i]) != 0 {
			utils.Panic("assignment for %s has not been assigned correctly: row %d CSV=%s got Wizard=%s", name, i, csvBi[i].String(), wizBi[i].String())
		}
	}

	for i := c.nbRows; i < len(wizBi); i++ {
		if !isZeroBigInt(wizBi[i]) {
			utils.Panic("assignment for %s has not been zero-padded correctly: row %d, got Wizard=%s", name, i, wizBi[i].String())
		}
	}
}

func (c *CsvTrace) Len() int {
	return c.nbRows
}

func (c *CsvTrace) LenPadded() int {
	return utils.NextPowerOfTwo(c.nbRows)
}

// WritesExplicit format value-provided columns into a csv file. Unlike [FmtCsv]
// it does not need the columns to be registered as the assignmet of a wizard.
// It is suitable for test-case generation.
func WriteExplicit(w io.Writer, names []string, cols [][]*big.Int, inHex bool) {

	fmt.Fprintf(w, "%v\n", strings.Join(names, ","))

	for i := range cols[0] {

		row := []string{}
		for j := range cols {
			row = append(row, fmtBigInt(inHex, cols[j][i]))
		}

		fmt.Fprintf(w, "%v\n", strings.Join(row, ","))
	}
}

func WriteExplicitFromKoala(w io.Writer, names []string, cols [][]field.Element, inHex bool) {

	fmt.Fprintf(w, "%v\n", strings.Join(names, ","))
	for i := range cols[0] {
		row := []string{}
		for j := range cols {
			var bi big.Int
			cols[j][i].BigInt(&bi)
			row = append(row, fmtBigInt(inHex, &bi))
		}
		fmt.Fprintf(w, "%v\n", strings.Join(row, ","))
	}
}

func fmtBigInt(inHex bool, x *big.Int) string {
	if !inHex || x.Uint64() < 1<<10 {
		return x.String()
	}
	return "0x" + x.Text(16)
}

func bigIntsToKoalaStrict(vBi []*big.Int) ([]field.Element, error) {
	vKoala := make([]field.Element, len(vBi))
	for i := range vBi {
		// Without this check, the [SetBigInt] method will happily
		// modulo reduce the provided value which would make debugging
		// more difficult.
		if vBi[i].Cmp(field.Modulus()) >= 0 {
			return nil, fmt.Errorf("value #%d, is %v which is greater than koalabear modulus (%v)", i, vBi[i], field.Modulus())
		}
		vKoala[i].SetBigInt(vBi[i])
	}
	return vKoala, nil
}

func koalaVecToBigInt(vKoala []field.Element) []*big.Int {
	vBi := make([]*big.Int, len(vKoala))
	for i := range vKoala {
		vBi[i] = new(big.Int)
		vBi[i] = vKoala[i].BigInt(vBi[i])
	}
	return vBi
}

func isZeroBigInt(bi *big.Int) bool {
	return bi.Cmp(big.NewInt(0)) == 0
}

func DebugProjection(p *query.Projection, run ifaces.Runtime) {
	FmtCsv(
		files.MustOverwrite(string(p.Name()+"-a.csv")),
		run,
		utils.SliceToAnys(p.Inp.ColumnsA[0]),
		nil,
	)
	FmtCsv(
		files.MustOverwrite(string(p.Name()+"-b.csv")),
		run,
		utils.SliceToAnys(p.Inp.ColumnsB[0]),
		nil,
	)
}
