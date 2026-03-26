package mempoolext

import (
	"errors"
	"fmt"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"
	"runtime"
	"strconv"
	"unsafe"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

type DebuggeableCall struct {
	Parent MemPool
	Logs   map[uintptr]*[]Record
}

func NewDebugPool(p MemPool) *DebuggeableCall {
	return &DebuggeableCall{
		Parent: p,
		Logs:   make(map[uintptr]*[]Record),
	}
}

type Record struct {
	Where string
	What  recordType
}

func (m *DebuggeableCall) Prewarm(nbPrewarm int) MemPool {
	m.Parent.Prewarm(nbPrewarm)
	return m
}

type recordType string

const (
	AllocRecord recordType = "alloc"
	FreeRecord  recordType = "free"
)

func (m *DebuggeableCall) Alloc() *[]fext.Element {

	var (
		v                = m.Parent.Alloc()
		uptr             = uintptr(unsafe.Pointer(v))
		logs             *[]Record
		_, file, line, _ = runtime.Caller(2)
	)

	logs, found := m.Logs[uptr]

	if !found {
		logs = &[]Record{}
		m.Logs[uptr] = logs
	}

	*logs = append(*logs, Record{
		Where: file + ":" + strconv.Itoa(line),
		What:  AllocRecord,
	})

	return v
}

func (m *DebuggeableCall) Free(v *[]fext.Element) error {

	var (
		uptr             = uintptr(unsafe.Pointer(v))
		logs             *[]Record
		_, file, line, _ = runtime.Caller(2)
	)

	logs, found := m.Logs[uptr]

	if !found {
		logs = &[]Record{}
		m.Logs[uptr] = logs
	}

	*logs = append(*logs, Record{
		Where: file + ":" + strconv.Itoa(line),
		What:  FreeRecord,
	})

	return m.Parent.Free(v)
}

func (m *DebuggeableCall) Size() int {
	return m.Parent.Size()
}

func (m *DebuggeableCall) TearDown() {
	if p, ok := m.Parent.(*SliceArena); ok {
		p.TearDown()
	}
}

func (m *DebuggeableCall) Errors() error {

	var err error

	for _, logs_ := range m.Logs {

		if logs_ == nil || len(*logs_) == 0 {
			utils.Panic("got a nil entry")
		}

		logs := *logs_

		for i := range logs {
			if i == 0 && logs[i].What == FreeRecord {
				err = errors.Join(err, fmt.Errorf("freed a vector that was not from the pool: where=%v", logs[i].Where))
			}

			if i == len(logs)-1 && logs[i].What == AllocRecord {
				err = errors.Join(err, fmt.Errorf("leaked a vector out of the pool: where=%v", logs[i].Where))
			}

			if i == 0 {
				continue
			}

			if logs[i-1].What == AllocRecord && logs[i].What == AllocRecord {
				wheres := []string{logs[i-1].Where, logs[i].Where}
				for k := i + 1; k < len(logs) && logs[k].What == AllocRecord; k++ {
					wheres = append(wheres, logs[k].Where)
				}

				err = errors.Join(err, fmt.Errorf("vector was allocated multiple times concurrently where=%v", wheres))
			}

			if logs[i-1].What == FreeRecord && logs[i].What == FreeRecord {
				wheres := []string{logs[i-1].Where, logs[i].Where}
				for k := i + 1; k < len(logs) && logs[k].What == FreeRecord; k++ {
					wheres = append(wheres, logs[k].Where)
				}

				err = errors.Join(err, fmt.Errorf("vector was freed multiple times concurrently where=%v", wheres))
			}
		}
	}

	return err
}
