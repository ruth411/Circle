package db

import (
	"context"
	"database/sql"
	"time"
)

func Open(driverName string, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}

type Tx interface {
	Commit() error
	Rollback() error
}

type BeginTxFunc func(context.Context, *sql.TxOptions) (Tx, error)

func BeginFromSQL(db *sql.DB) BeginTxFunc {
	return func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
		return db.BeginTx(ctx, opts)
	}
}

func WithTx(ctx context.Context, begin BeginTxFunc, opts *sql.TxOptions, fn func(context.Context, Tx) error) (err error) {
	tx, err := begin(ctx, opts)
	if err != nil {
		return err
	}

	defer func() {
		if recoverValue := recover(); recoverValue != nil {
			_ = tx.Rollback()
			panic(recoverValue)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = fn(ctx, tx); err != nil {
		return err
	}

	return tx.Commit()
}
