package notificationbot

type DiscordNotificationSender interface {
	SendNotification(msg, notificationType string) error
}
