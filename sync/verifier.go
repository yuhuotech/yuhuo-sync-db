package sync

import (
	"fmt"

	"github.com/yuhuo/sync-db/database"
)

// Verifier 用于验证同步后的结果
type Verifier struct {
	comparator *Comparator
}

// NewVerifier 创建验证器
func NewVerifier(sourceConn, targetConn *database.Connection) *Verifier {
	return &Verifier{
		comparator: NewComparator(sourceConn, targetConn),
	}
}

// VerifySync 验证同步结果
func (v *Verifier) VerifySync(syncDataTables []string) (bool, string, error) {
	// 重新比对差异
	diff, err := v.comparator.CompareDifferences(syncDataTables)
	if err != nil {
		return false, "", fmt.Errorf("failed to verify sync: %w", err)
	}

	// 如果没有差异，则同步成功
	if !diff.HasDifferences() {
		return true, "Sync verification passed! No differences found.", nil
	}

	// 如果还有差异，则同步未完成
	message := fmt.Sprintf("Sync verification failed! Still have differences:\n"+
		"Structure differences: %d\n"+
		"Data differences: %d\n"+
		"View differences: %d",
		len(diff.StructureDifferences),
		len(diff.DataDifferences),
		len(diff.ViewDifferences))

	return false, message, nil
}
