package workers

import (
	"context"
	"diplom_ya/internal/config"
	"diplom_ya/internal/store"
	"fmt"
	"log"
)

// Вычитать заказы со статусами new, registered, processing
func WriteOrderProcessing(ctx context.Context, cfg config.Config) {
	orders, err := store.GetOrdersProcessing(ctx, cfg)
	if err != nil {
		log.Println(err)
	}
	for _, number := range orders {
		cfg.ChanOrdersProc <- number
	}
}

// обработать заказы из канала
func OrderProcessing(ctx context.Context, cfg config.Config) {

	for order := range cfg.ChanOrdersProc {
		fmt.Println(order)
	}

}

func CloseWorkers(cfg config.Config) {
	close(cfg.ChanOrdersProc)
}

func StartWorkers(cfg config.Config) {
	go WriteOrderProcessing(context.Background(), cfg)
	go OrderProcessing(context.Background(), cfg)
}
