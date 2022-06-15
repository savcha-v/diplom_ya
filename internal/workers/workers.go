package workers

import (
	"context"
	"diplom_ya/internal/config"
)

func CloseWorkers(cfg config.Config) {
	close(cfg.ChanOrdersProc)
}

func StartWorkers(cfg config.Config) {
	go WriteOrderProcessing(context.Background(), cfg)
	go ReadOrderProcessing(context.Background(), cfg)
}
