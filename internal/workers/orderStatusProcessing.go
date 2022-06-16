package workers

import (
	"context"
	"diplom_ya/internal/config"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

type orderData struct {
	Order  string `json:"order"`
	Status string `json:"status"`
	Sum    int    `json:"accrual"`
	UserID string
}

func AddOrderToChannelProc(cfg config.Config, number string) {
	cfg.ChanOrdersProc <- number
}

// записать в канал заказы со статусами new, registered, processing
func WriteOrderProcessing(ctx context.Context, cfg config.Config) {
	orders, err := getOrdersProcessing(ctx, cfg)
	if err != nil {
		log.Println(err)
	}
	for _, number := range orders {
		AddOrderToChannelProc(cfg, number)
	}
}

// обработать заказы из канала
func ReadOrderProcessing(ctx context.Context, cfg config.Config) {

	for number := range cfg.ChanOrdersProc {
		orderData, err := getOrderData(ctx, cfg, number)
		if err != nil {
			log.Println(err)
			AddOrderToChannelProc(cfg, number)
			return
		}

		status, err := updateOrder(ctx, cfg, orderData)
		if err != nil {
			log.Println(err)
			AddOrderToChannelProc(cfg, number)
			return
		}

		// если не в конечном статусе
		if status != cfg.OrdersStatus.Processed && status != cfg.OrdersStatus.Invalid {
			go AddOrderToChannelProc(cfg, number)
		}

	}
}

func getOrderData(ctx context.Context, cfg config.Config, number string) (orderData, error) {

	fmt.Fprintln(os.Stdout, "getOrderData")

	valueIn := orderData{}

	addressCalc := cfg.AccrualAddress + "/api/orders/" + number
	r, err := http.Get(addressCalc)
	if err != nil {
		return valueIn, errors.New("error call /api/orders/")
	}

	// if r.StatusCode == http.StatusTooManyRequests {
	// 	fmt.Fprintln(os.Stdout, "StatusTooManyRequests")
	// 	retryHead := r.Header.Get("Retry-After")
	// 	if retryHead != "" {
	// 		retry, err := strconv.Atoi(retryHead)
	// 		if err != nil {
	// 			return valueIn, errors.New("error conv Retry-After /api/orders/")
	// 		}
	// 		time.Sleep(time.Duration(retry) * time.Second)

	// 		return valueIn, errors.New("getOrderData/ wait retry /api/orders/")
	// 	}
	// }

	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Fprintln(os.Stdout, err)
		return valueIn, errors.New("error read body /api/orders/")
	}

	fmt.Fprintln(os.Stdout, "getOrderData/ body")
	fmt.Fprintln(os.Stdout, string(body))
	defer r.Body.Close()

	if err := json.Unmarshal(body, &valueIn); err != nil {
		return valueIn, errors.New("error unmarshal /api/orders/")
	}

	if valueIn.Order == "" {
		return valueIn, errors.New("error unmarshal valueIn.Order is empty /api/orders/")
	}

	userID, err := getUserID(ctx, cfg, number)

	if err != nil {
		return valueIn, errors.New("error get user ID /api/orders/")
	}

	valueIn.UserID = userID

	// valueIn.UserID = `1c2be014-8880-4e33-aa94-6a3986253b0c`
	// valueIn.Status = "PROCESSED"
	// valueIn.Sum = "200"
	// valueIn.Order = number

	return valueIn, nil
}

func getUserID(ctx context.Context, cfg config.Config, number string) (string, error) {

	fmt.Fprintln(os.Stdout, "getUserID")

	db := cfg.ConnectDB

	textQuery := `SELECT "userID" FROM accum WHERE "order" = $1`
	rows, err := db.QueryContext(ctx, textQuery, number)

	if err != nil {
		return "", errors.New("error get userID")
	}
	defer rows.Close()

	var userID string

	for rows.Next() {
		err = rows.Scan(&userID)
		if err != nil {
			return "", errors.New("error scan rows in db")
		}
	}

	err = rows.Err()
	if err != nil {
		return "", errors.New("rows error in db")
	}

	return userID, nil
}

func getOrdersProcessing(ctx context.Context, cfg config.Config) ([]string, error) {

	fmt.Fprintln(os.Stdout, "getOrdersProcessing")

	db := cfg.ConnectDB

	textQuery := `SELECT "order"
	FROM  accum 
	where "status" = $1 or "status" = $2 or "status" = $3`

	var out []string
	// new, registered, processing
	rows, err := db.QueryContext(ctx, textQuery, cfg.OrdersStatus.New, cfg.OrdersStatus.Processing, cfg.OrdersStatus.Registered)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item string
		err = rows.Scan(&item)
		if err != nil {
			return nil, err
		}

		out = append(out, item)
	}

	return out, err
}

func updateOrder(ctx context.Context, cfg config.Config, data orderData) (string, error) {

	fmt.Fprintln(os.Stdout, "updateOrder")

	db := cfg.ConnectDB

	mu := &sync.Mutex{}
	mu.Lock()
	defer mu.Unlock()

	// Начало транзацкции
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if data.Status == cfg.OrdersStatus.Processed {

		textQuery := `UPDATE accum SET "sum" = $1, "status" = $2 WHERE "order" = $3`
		_, err = tx.ExecContext(ctx, textQuery, data.Sum, data.Status, data.Order)

		if err != nil {
			return "", err
		}

		textQuery = `UPDATE users SET "balanse" = "balanse" + $1 WHERE "userID" = $2`
		_, err = tx.ExecContext(ctx, textQuery, data.Sum, data.UserID)
		if err != nil {
			return "", err
		}

	} else {
		textQuery := `UPDATE accum SET "status" = $1 WHERE "order" = $2`
		_, err = tx.ExecContext(ctx, textQuery, data.Status, data.Order)
		if err != nil {
			return "", err
		}
	}

	tx.Commit()

	return data.Status, nil
}
