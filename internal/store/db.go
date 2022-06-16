package store

import (
	"context"
	"database/sql"
	"diplom_ya/internal/config"
	"diplom_ya/internal/workers"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v4/stdlib"
)

func DBInit(cfg *config.Config) error {

	db, err := sql.Open("pgx", cfg.DataBase)
	if err != nil {
		return err
	}

	// users
	textCreate := `CREATE TABLE IF NOT EXISTS users(
		"userID" TEXT,
		"login" TEXT PRIMARY KEY,
		"hash" TEXT,
		"balanse" FLOAT 
		 );`
	if _, err := db.Exec(textCreate); err != nil {
		return err
	}
	// accumulation
	textCreate = `CREATE TABLE IF NOT EXISTS accum(
		"userID" TEXT,
		"order" TEXT PRIMARY KEY,
		"sum" FLOAT,
		"date" DATE,
		"status" TEXT
		 );`
	if _, err := db.Exec(textCreate); err != nil {
		return err
	}
	// subtraction
	textCreate = `CREATE TABLE IF NOT EXISTS subtract(
		"userID" TEXT,
		"order" TEXT PRIMARY KEY,
		"sum" FLOAT,
		"date" DATE
		 );`
	if _, err := db.Exec(textCreate); err != nil {
		return err
	}

	cfg.ConnectDB = db
	return nil
}

func LoginUse(ctx context.Context, cfg config.Config, login string) (bool, error) {
	var userID string

	db := cfg.ConnectDB

	textQuery := `SELECT "userID" FROM users WHERE "login" = $1`
	err := db.QueryRowContext(ctx, textQuery, login).Scan(&userID)

	switch {
	case err == sql.ErrNoRows:
		return false, nil
	case err != nil:
		return false, err
	default:
		return true, nil
	}
}

func WriteNewUser(ctx context.Context, cfg config.Config, login string, hash string) (string, error) {

	db := cfg.ConnectDB

	userID := uuid.New().String()
	textInsert := `
	INSERT INTO users ("userID", "login", "hash", "balanse")
	VALUES ($1, $2, $3, $4)`
	_, err := db.ExecContext(ctx, textInsert, userID, login, hash, 0)

	if err != nil {
		return "", err
	}

	return userID, nil
}

func ReadUser(ctx context.Context, cfg config.Config, login string, hash string) (string, error) {
	var userID string

	db := cfg.ConnectDB

	textQuery := `SELECT "userID" FROM users WHERE "login" = $1 AND "hash" = $2`
	err := db.QueryRowContext(ctx, textQuery, login, hash).Scan(&userID)

	switch {
	case err == sql.ErrNoRows:
		return "", nil
	case err != nil:
		return "", err
	default:
		return userID, nil
	}
}

func ExistsUserID(ctx context.Context, cfg config.Config, userID string) (bool, error) {
	var login string

	db := cfg.ConnectDB

	textQuery := `SELECT "login" FROM users WHERE "userID" = $1`
	err := db.QueryRowContext(ctx, textQuery, userID).Scan(&login)

	switch {
	case err == sql.ErrNoRows:
		return false, nil
	case err != nil:
		return false, err
	default:
		return true, nil
	}
}

func GetBalanseSpent(ctx context.Context, cfg config.Config, userID string) (balance float32, spent float32, err error) {

	db := cfg.ConnectDB

	textQuery := `SELECT max(users."balanse"), sum(COALESCE(subtract."sum",0))
	FROM users left join subtract on users."userID" = subtract."userID"
	where users."userID" = $1`

	err = db.QueryRowContext(ctx, textQuery, userID).Scan(&balance, &spent)
	return
}

func AddOrder(ctx context.Context, cfg config.Config, order string, userID string) int {
	mu := &sync.Mutex{}
	mu.Lock()
	defer mu.Unlock()

	var receivedUserID string

	db := cfg.ConnectDB

	textQuery := `SELECT "userID" FROM accum WHERE "order" = $1`
	err := db.QueryRowContext(ctx, textQuery, order).Scan(&receivedUserID)

	switch {
	case err == sql.ErrNoRows:
		// add in db
		textInsert := `
		INSERT INTO accum ("userID", "order", "sum", "date", "status")
		VALUES ($1, $2, $3, $4, $5)`
		_, err = db.ExecContext(ctx, textInsert, userID, order, 0, time.Now(), cfg.OrdersStatus.New)

		if err != nil {
			return http.StatusInternalServerError
		}

		workers.AddOrderToChannelProc(cfg, order)

		return http.StatusAccepted
	case err != nil:
		return http.StatusInternalServerError
	case receivedUserID != userID:
		return http.StatusConflict
	default:
		return http.StatusOK
	}
}

func GetAccum(ctx context.Context, cfg config.Config, userID string) ([]config.OutAccum, error) {

	db := cfg.ConnectDB

	textQuery := `SELECT "order", "sum", "date", "status"
	FROM  accum 
	where "userID" = $1 ORDER BY "date"`

	var out []config.OutAccum

	rows, err := db.QueryContext(ctx, textQuery, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	err = rows.Err()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var item config.OutAccum
		err = rows.Scan(&item.Order, &item.Sum, &item.Date, &item.Status)
		if err != nil {
			return nil, err
		}

		out = append(out, item)
	}

	return out, err
}

func WriteWithdraw(ctx context.Context, cfg config.Config, order string, sum float32, userID string) int {

	mu := &sync.Mutex{}
	mu.Lock()
	defer mu.Unlock()

	balance, _, err := GetBalanseSpent(ctx, cfg, userID)
	if err != nil {
		return http.StatusInternalServerError
	}
	if balance < sum {
		return http.StatusPaymentRequired
	}

	db := cfg.ConnectDB

	// add in db
	textInsert := `
		INSERT INTO subtract ("userID", "order", "sum", "date")
		VALUES ($1, $2, $3, $4, $5)`
	_, err = db.ExecContext(ctx, textInsert, userID, order, sum, time.Now())

	if err != nil {
		return http.StatusInternalServerError
	}
	return http.StatusOK
}

func GetWithdrawals(ctx context.Context, cfg config.Config, userID string) ([]config.OutWithdrawals, error) {

	db := cfg.ConnectDB

	textQuery := `SELECT "order", "sum", "date"
	FROM  subtruct 
	where "userID" = $1 ORDER BY "date"`

	var out []config.OutWithdrawals

	rows, err := db.QueryContext(ctx, textQuery, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item config.OutWithdrawals
		err = rows.Scan(&item.Order, &item.Sum, &item.Date)
		if err != nil {
			return nil, err
		}

		out = append(out, item)
	}

	return out, err
}
