package service

import (
	"context"
	"database/sql"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

func (s *SiteService) RunNotificationConsumer(logger *logrus.Entry, ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	l := logger.WithField("serviceName", "notificationConsumer")
	defer l.Info("consumer stopped")

	bucket, ticker := utils.NewBucketLimiter(10*time.Millisecond, 1)
	defer ticker.Stop()

	s.announceNotification()

	const errorSleepTime = time.Second * 60

	for {
		select {
		case <-ctx.Done():
			l.Info("context cancelled, stopping notification consumer")
			return
		case <-s.notificationQueueNotEmpty:
			select {
			case <-ctx.Done():
				l.Info("context cancelled, stopping notification consumer")
				return
			case <-bucket:
			}

			// TODO yea, like this is fetching notifications one by one, which is lovely and simple,
			// but also has some room for optimizing database access

			loopWrap := func() {
				dbs, err := s.dal.NewSession(ctx)
				if err != nil {
					if err == context.Canceled {
						return
					}
					l.Error(err)
					l.Debugf("sleeping for %f seconds", errorSleepTime.Seconds())
					time.Sleep(errorSleepTime)
					return
				}
				defer dbs.Rollback()

				notification, err := s.dal.GetOldestUnsentNotification(dbs)
				if err != nil {
					if err == context.Canceled {
						return
					}
					if err == sql.ErrNoRows {
						l.Debug("notification queue is empty, waiting for announcement to resume consumption")
						return
					}
					l.Error(err)
					l.Debugf("sleeping for %f seconds", errorSleepTime.Seconds())
					time.Sleep(errorSleepTime)
					return
				}
				s.announceNotification()

				if err := s.notificationBot.SendNotification(notification.Message, notification.Type); err != nil {
					l.Error(err)
					l.Debugf("sleeping for %f seconds", errorSleepTime.Seconds())
					time.Sleep(errorSleepTime)
					return
				}

				if err := s.dal.MarkNotificationAsSent(dbs, notification.ID); err != nil {
					if err == context.Canceled {
						return
					}
					l.Error(err)
					l.Debugf("sleeping for %f seconds", errorSleepTime.Seconds())
					time.Sleep(errorSleepTime)
					return
				}

				if err := dbs.Commit(); err != nil {
					if err == context.Canceled {
						return
					}
					l.Error(err)
					l.Debugf("sleeping for %f seconds", errorSleepTime.Seconds())
					time.Sleep(errorSleepTime)
					return
				}
			}

			loopWrap()
		}
	}
}

func (s *SiteService) announceNotification() {
	select {
	// non-blocking announce that something is in the queue
	case s.notificationQueueNotEmpty <- true:
	default:
	}
}
