package config

import (
	"database/sql"
	"flag"
	"log"
	"time"

	"github.com/caarlos0/env"
)

type Config struct {
	RunAddress     string `env:"RUN_ADDRESS" envDefault:"localhost:9090"`
	DataBase       string `env:"DATABASE_URI"`
	AccrualAddress string `env:"ACCRUAL_SYSTEM_ADDRESS" envDefault:"http://localhost:8080"`
	ConnectDB      *sql.DB
	Key            string
	OrdersStatus
	ChanOrdersProc chan string
}

type OrdersStatus struct {
	New        string // заказ загружен в систему, но не попал в обработку;
	Processing string // вознаграждение за заказ рассчитывается;
	Invalid    string // система расчёта вознаграждений отказала в расчёте;
	Processed  string // данные по заказу проверены и информация о расчёте успешно получена.
	Registered string // заказ зарегистрирован, но не начисление не рассчитано;
}

type OutAccum struct {
	Order  string    `json:"number"`
	Status string    `json:"status"`
	Sum    float32   `json:"accrual,omitempty"`
	Date   time.Time `json:"uploaded_at"`
}

type OutWithdrawals struct {
	Order string    `json:"order"`
	Sum   float32   `json:"accrual"`
	Date  time.Time `json:"processed_at"`
}

func New() Config {

	var cfg Config

	cfg.Key = "10c57de0"

	if err := env.Parse(&cfg); err != nil {
		log.Fatal(err)
	}

	flag.StringVar(&cfg.RunAddress, "a", cfg.RunAddress, "")
	flag.StringVar(&cfg.DataBase, "d", cfg.DataBase, "")
	flag.StringVar(&cfg.AccrualAddress, "r", cfg.AccrualAddress, "")

	flag.Parse()

	statuses := OrdersStatus{
		New:        "NEW",
		Processing: "PROCESSING",
		Invalid:    "INVALID",
		Processed:  "PROCESSED",
		Registered: "REGISTERED ",
	}

	cfg.OrdersStatus = statuses

	cfg.ChanOrdersProc = make(chan string)

	return cfg
}
