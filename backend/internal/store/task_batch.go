package store

import (
	"context"
	"os"
	"strings"
	"time"

	"emosup/backend/internal/model"
	"emosup/backend/internal/utils"
)

type TaskBatchCreateFailure struct {
	ItemID string `json:"item_id"`
	Reason string `json:"reason"`
}

type TaskBatchCreateResult struct {
	Created []model.Task
	Failed  []TaskBatchCreateFailure
}

// CreateTasksForScan inserts tasks and their initial log entries in the same
// transaction as scan-item cleanup, so a partial queue can never be left behind.
func (s *FileStore) CreateTasksForScan(ctx context.Context, scanID string, tasks []model.Task) (TaskBatchCreateResult, error) {
	result := TaskBatchCreateResult{
		Created: make([]model.Task, 0, len(tasks)),
		Failed:  make([]TaskBatchCreateFailure, 0),
	}
	if len(tasks) == 0 {
		return result, nil
	}

	tx, err := s.beginWrite(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()

	var scanExists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM scans WHERE id = ?`, scanID).Scan(&scanExists); err != nil {
		return result, err
	}
	if scanExists == 0 {
		return result, os.ErrNotExist
	}

	activeByItemID := make(map[string]struct{})
	rows, err := tx.QueryContext(ctx, `
SELECT scan_item_id, status FROM tasks
WHERE scan_session_id = ? AND scan_item_id <> ''`, scanID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var itemID, status string
		if err := rows.Scan(&itemID, &status); err != nil {
			_ = rows.Close()
			return result, err
		}
		if status == string(model.TaskStatusCompleted) || status == string(model.TaskStatusCanceled) {
			continue
		}
		activeByItemID[itemID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return result, err
	}
	if err := rows.Close(); err != nil {
		return result, err
	}

	seen := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		itemID := strings.TrimSpace(task.ScanItemID)
		if itemID == "" {
			result.Failed = append(result.Failed, TaskBatchCreateFailure{ItemID: itemID, Reason: "scan item id is empty"})
			continue
		}
		if _, ok := seen[itemID]; ok {
			result.Failed = append(result.Failed, TaskBatchCreateFailure{ItemID: itemID, Reason: "duplicate item id in request"})
			continue
		}
		if _, ok := activeByItemID[itemID]; ok {
			result.Failed = append(result.Failed, TaskBatchCreateFailure{ItemID: itemID, Reason: "active task already exists for scan item"})
			continue
		}
		if err := upsertTaskTx(ctx, tx.Tx, task); err != nil {
			return result, err
		}
		if err := insertLogTx(ctx, tx.Tx, task.ID, model.TaskLogItem{
			ID:      utils.NewID("log"),
			Level:   "info",
			Message: "task created from scan item",
			Time:    time.Now(),
		}); err != nil {
			return result, err
		}

		seen[itemID] = struct{}{}
		activeByItemID[itemID] = struct{}{}
		result.Created = append(result.Created, task)
	}

	for _, task := range result.Created {
		if _, err := tx.ExecContext(ctx, `DELETE FROM scan_items WHERE scan_session_id = ? AND id = ?`, scanID, task.ScanItemID); err != nil {
			return result, err
		}
	}

	if len(result.Created) > 0 {
		if err := refreshScanCountsTx(ctx, tx.Tx, scanID); err != nil {
			return result, err
		}
		var remaining int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM scan_items WHERE scan_session_id = ?`, scanID).Scan(&remaining); err != nil {
			return result, err
		}
		if remaining == 0 {
			if _, err := tx.ExecContext(ctx, `DELETE FROM scans WHERE id = ?`, scanID); err != nil {
				return result, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

// CreateManualTasks inserts manually created tasks and their initial log entries directly into the database.
func (s *FileStore) CreateManualTasks(ctx context.Context, tasks []model.Task) (TaskBatchCreateResult, error) {
	result := TaskBatchCreateResult{
		Created: make([]model.Task, 0, len(tasks)),
		Failed:  make([]TaskBatchCreateFailure, 0),
	}
	if len(tasks) == 0 {
		return result, nil
	}

	tx, err := s.beginWrite(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()

	for _, task := range tasks {
		if err := upsertTaskTx(ctx, tx.Tx, task); err != nil {
			return result, err
		}
		if err := insertLogTx(ctx, tx.Tx, task.ID, model.TaskLogItem{
			ID:      utils.NewID("log"),
			Level:   "info",
			Message: "manual task created",
			Time:    time.Now(),
		}); err != nil {
			return result, err
		}
		result.Created = append(result.Created, task)
	}

	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

