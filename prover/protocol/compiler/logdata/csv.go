package logdata

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// CSVFilterOptions are flags to provide to the csv generator
type CSVFilterOptions uint64

const (
	IncludeNonIgnoredColumnCSVFilter CSVFilterOptions = 1 << iota
	IncludeIgnoredColumnCSVFilter
	IncludeNonIgnoredQueryCSVFilter
	IncludeIgnoredQueryCSVFilter
	IncludeCoinCSVFilter
	IncludeVerificationKeyCsvFilter
	IncludeAllFilter       CSVFilterOptions = 0xffffffffffffffff
	IncludeColumnCSVFilter                  = IncludeIgnoredColumnCSVFilter | IncludeNonIgnoredColumnCSVFilter
	IncludeQueryCSVFilter                   = IncludeNonIgnoredQueryCSVFilter | IncludeIgnoredQueryCSVFilter
)

// Dump the columns into a csv file
func GenCSV(w io.Writer, filter CSVFilterOptions) func(comp *wizard.CompiledIOP) {

	return func(comp *wizard.CompiledIOP) {

		io.WriteString(w, "name; size; status; round; type\n")

		if filter&IncludeColumnCSVFilter > 0 {

			columns := comp.Columns.AllKeys()
			for _, colID := range columns {

				var (
					col    = comp.Columns.GetHandle(colID)
					status = comp.Columns.Status(colID)
					row    = &csvRow{
						size:   col.Size(),
						round:  col.Round(),
						status: status.String(),
						typ:    "Column",
						id:     col.String(),
					}
				)

				if status == column.Ignored && filter&IncludeIgnoredColumnCSVFilter == 0 {
					continue
				}

				if status != column.Ignored && filter&IncludeNonIgnoredColumnCSVFilter == 0 {
					continue
				}

				row.Write(w)
			}
		}

		if filter&IncludeQueryCSVFilter > 0 {

			registers := []wizard.ByRoundRegister[ifaces.QueryID, ifaces.Query]{
				comp.QueriesNoParams,
				comp.QueriesParams,
			}

			for _, reg := range registers {

				queries := reg.AllKeys()
				for i := range queries {

					var (
						name      = queries[i]
						q         = reg.Data(name)
						isIgnored = reg.IsIgnored(name)
						status    = utils.Ternary(isIgnored, "Ignored", "Active")
						round     = reg.Round(name)
						row       = &csvRow{
							status: status,
							round:  round,
							id:     string(name),
							typ:    reflect.TypeOf(q).Name(),
						}
					)

					if isIgnored && filter&IncludeIgnoredQueryCSVFilter == 0 {
						continue
					}

					if !isIgnored && filter&IncludeNonIgnoredQueryCSVFilter == 0 {
						continue
					}

					row.SetQuery(q)
					row.Write(w)
				}
			}
		}

		if filter&IncludeCoinCSVFilter > 0 {

			coins := comp.Coins.AllKeys()
			for _, c := range coins {

				info := comp.Coins.Data(c)
				row := &csvRow{
					round:  comp.Coins.Round(c),
					id:     info.String(),
					status: "-",
					typ:    strconv.Itoa(int(info.Type)),
					size:   info.Size,
				}

				row.Write(w)
			}
		}

		if filter&IncludeVerificationKeyCsvFilter > 0 {

			for round := 0; round < comp.NumRounds(); round++ {

				subV := comp.GetSubVerifiers()
				vas := subV.GetOrEmpty(round)
				for i := range vas {

					va := vas[i]
					row := &csvRow{
						round:  round,
						id:     reflect.TypeOf(va).String(),
						status: "-",
						typ:    "VerifierAction",
						size:   0,
					}

					row.Write(w)
				}
			}
		}

		for _, pubInputs := range comp.PublicInputs {
			row := &csvRow{
				round:  0,
				id:     pubInputs.Name,
				status: "-",
				typ:    "PublicInput",
				size:   0,
			}

			row.Write(w)
		}
	}
}

// Represents a csv row to print
type csvRow struct {
	id     string
	size   int
	status string
	round  int
	typ    string
	val    string
	extra  []string
}

func (r *csvRow) SetQuery(q ifaces.Query) {

	r.typ = reflect.TypeOf(q).Name()

	switch q_ := q.(type) {
	case query.LogDerivativeSum:
		r.size = 1
	case query.GrandProduct:
		r.size = 1
	case query.LocalOpening:
		r.size = 1
	case query.UnivariateEval:
		r.size = len(q_.Pols)
		extras := make([]string, len(q_.Pols))
		r.extra = extras
	case query.InnerProduct:
		r.size = len(q_.Bs)
	case *query.Horner:
		r.size = 1 + 2*len(q_.Parts)
	}
}

func (r *csvRow) Write(w io.Writer) {
	fmt.Fprintln(w, r.id, ";", r.typ, ";", r.status, ";", r.round, ";", r.size, ";", r.val, ";", r.extra)
}
