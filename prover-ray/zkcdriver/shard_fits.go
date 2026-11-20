package zkcdriver

import (
	"errors"
	"fmt"
	"strings"
)

// CheckShardFits returns an error naming every module of the shard with more
// rows than maxRows, the most rows a column can hold
// ([wiop.ColumnSizeMaxSupported] in a real system). Assigning such a shard
// would panic in a worker goroutine, which no caller can recover. Static
// modules are skipped, as the assignment skips them.
func (a *ZkCDriver) CheckShardFits(shard Shard, maxRows uint) error {
	schema := a.BinaryFile.AirConstraints()
	var over []string
	for k := range shard.Width() {
		if schema.Module(k).IsStatic() {
			continue
		}
		if m := shard.Module(k); m.Height() > maxRows {
			over = append(over, fmt.Sprintf("module %s has %d rows, more than the %d a column can hold",
				m.Name(), m.Height(), maxRows))
		}
	}
	if len(over) == 0 {
		return nil
	}
	return errors.New(strings.Join(over, "; "))
}
