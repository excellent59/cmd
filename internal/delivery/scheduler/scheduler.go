package scheduler

import (
	"context"
	"time"

	"pro-lot-bot/internal/delivery/telegram"
	"pro-lot-bot/internal/domain"
	"pro-lot-bot/internal/logger"
)

// PollProvider отдаёт актуальные настройки, чтобы менять частоту опроса на лету.
type PollProvider interface {
	GetSettings(ctx context.Context) (domain.Settings, error)
}

type Scheduler struct {
	handler  *telegram.BotHandler
	poll     PollProvider
	interval time.Duration
}

func NewScheduler(handler *telegram.BotHandler, poll PollProvider, interval time.Duration) *Scheduler {
	return &Scheduler{
		handler:  handler,
		poll:     poll,
		interval: interval,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	log := logger.Get()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Первый запуск сразу
	s.run(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info("Scheduler stopped")
			return
		case <-ticker.C:
			s.run(ctx)

			// Подхватываем интервал, изменённый через админку.
			if st, err := s.poll.GetSettings(ctx); err == nil && st.PollInterval > 0 && st.PollInterval != s.interval {
				log.Infow("Интервал опроса обновлён через админку",
					"old", s.interval.String(), "new", st.PollInterval.String())
				s.interval = st.PollInterval
				ticker.Reset(s.interval)
			}
		}
	}
}

func (s *Scheduler) run(ctx context.Context) {
	log := logger.Get()
	log.Info("Checking for new lots...")

	if err := s.handler.ProcessAndSendLots(ctx); err != nil {
		log.Errorw("Error processing lots", "error", err)
	}
}
