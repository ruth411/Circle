package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

type fakeTx struct {
	committed  bool
	rolledBack bool
	commitErr  error
}

func (f *fakeTx) Commit() error {
	f.committed = true
	return f.commitErr
}

func (f *fakeTx) Rollback() error {
	f.rolledBack = true
	return nil
}

func TestWithTxCommitsOnSuccess(t *testing.T) {
	tx := &fakeTx{}
	begin := func(context.Context, *sql.TxOptions) (Tx, error) {
		return tx, nil
	}

	err := WithTx(context.Background(), begin, nil, func(context.Context, Tx) error {
		return nil
	})
	if err != nil {
		t.Fatalf("WithTx returned error: %v", err)
	}
	if !tx.committed {
		t.Fatal("commit was not called")
	}
	if tx.rolledBack {
		t.Fatal("rollback should not be called on success")
	}
}

func TestWithTxRollsBackOnFailure(t *testing.T) {
	tx := &fakeTx{}
	begin := func(context.Context, *sql.TxOptions) (Tx, error) {
		return tx, nil
	}

	wantErr := errors.New("boom")
	err := WithTx(context.Background(), begin, nil, func(context.Context, Tx) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if !tx.rolledBack {
		t.Fatal("rollback was not called")
	}
	if tx.committed {
		t.Fatal("commit should not be called on failure")
	}
}
