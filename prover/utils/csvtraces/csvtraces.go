// Package csvtraces provides a way to read and write traces in CSV format.
package csvtraces

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

type cfg struct {
	// The number of rows in the trace
	nbRows             int
	skipPrePaddingZero bool
	filterOn           ifaces.Column
	inHex              bool
}

type Option func(*cfg) error

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

type CsvTrace struct {
	mapped map[string][]field.Element

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
// in a csv format so that debugging and testcase generation are simpler.
func FmtCsv(w io.Writer, run *wizard.ProverRuntime, cols []ifaces.Column, options []Option) error {

	var (
		header       = []string{}
		assignment   = [][]field.Element{}
		cfg          = cfg{}
		foundNonZero = false
		filterCol    []field.Element
	)

	for _, op := range options {
		op(&cfg)
	}

	for i := range cols {
		header = append(header, string(cols[i].GetColID()))
		assignment = append(assignment, cols[i].GetColAssignment(run).IntoRegVecSaveAlloc())
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

			if !assignment[c][r].IsZero() {
				allZeroes = false
			}

			if assignment[c][r].IsUint64() {
				if cfg.inHex {
					fmtVals = append(fmtVals, "0x"+assignment[c][r].Text(16))
				} else {
					fmtVals = append(fmtVals, assignment[c][r].String())
				}
				continue
			}

			fmtVals = append(fmtVals, "0x"+assignment[c][r].Text(16))
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

	data := make(map[string][]field.Element)
	header, err := rr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header row: %w", err)
	}
	for _, h := range header {
		data[h] = make([]field.Element, 0)
	}
	var nbRows int
	for row, err := rr.Read(); err != io.EOF; row, err = rr.Read() {
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		for i, h := range header {
			data[h] = append(data[h], field.NewFromString(row[i]))
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

func (c *CsvTrace) Get(name string) []field.Element {
	val, ok := c.mapped[name]
	if !ok {
		utils.Panic("column not found %s", name)
	}
	return val
}

func (c *CsvTrace) GetCommit(b *wizard.Builder, name string) ifaces.Column {
	if _, ok := c.mapped[name]; !ok {
		utils.Panic("column not found %s", name)
	}
	length := utils.NextPowerOfTwo(c.nbRows)
	col := b.RegisterCommit(ifaces.ColID(name), length)
	return col
}

func (c *CsvTrace) Assign(run *wizard.ProverRuntime, names ...string) {
	length := utils.NextPowerOfTwo(c.nbRows)
	for _, k := range names {
		if v, ok := c.mapped[k]; ok {
			sv := smartvectors.RightZeroPadded(v, length)
			run.AssignColumn(ifaces.ColID(k), sv)
		} else {
			utils.Panic("column not found %s", k)
		}
	}
}

func (c *CsvTrace) CheckAssignment(run *wizard.ProverRuntime, names ...string) {
	for _, name := range names {
		c.checkAssignment(run, name)
	}
}

func (c *CsvTrace) checkAssignment(run *wizard.ProverRuntime, name string) {
	colId := ifaces.ColID(name)
	assigned := run.Spec.Columns.GetHandle(colId)
	c.CheckAssignmentColumn(run, name, assigned)

}

func (c *CsvTrace) CheckAssignmentColumn(run *wizard.ProverRuntime, name string, col ifaces.Column) {

	var (
		stored, ok = c.mapped[name]
		assigned   = col.GetColAssignment(run)
		fullLength = utils.NextPowerOfTwo(c.nbRows)
	)

	if !ok {
		utils.Panic("column not found in CSV: %s", name)
	}

	if assigned.Len() < fullLength {
		utils.Panic("column %s has not been assigned with the expected length, found %v in CSV and %v in wizard", name, fullLength, assigned.Len())
	}

	vec := assigned.IntoRegVecSaveAlloc()
	for i := 0; i < c.nbRows; i++ {
		if vec[i].Cmp(&stored[i]) != 0 {
			utils.Panic("column %s has not been assigned correctly: row %d CSV=%s got Wizard=%s", name, i, stored[i].String(), vec[i].String())
		}
	}

	for i := c.nbRows; i < assigned.Len(); i++ {
		if !vec[i].IsZero() {
			utils.Panic("column %s is not properly zero-padded", name)
		}
	}
}

func (c *CsvTrace) Len() int {
	return c.nbRows
}

func (c *CsvTrace) LenPadded() int {
	return utils.NextPowerOfTwo(c.nbRows)
}
