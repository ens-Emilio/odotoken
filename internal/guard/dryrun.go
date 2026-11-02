package guard

import "sync/atomic"

var dryRunBlocks atomic.Int64

// IncrementDryRunBlocks records a would-be block that was allowed through dry-run mode.
func IncrementDryRunBlocks() {
	dryRunBlocks.Add(1)
}

// GetDryRunBlocks returns the total count of dry-run blocks.
func GetDryRunBlocks() int64 {
	return dryRunBlocks.Load()
}