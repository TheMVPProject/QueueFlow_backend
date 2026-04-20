package services

import (
	"context"
	"fmt"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type FCMService struct {
	client *messaging.Client
}

func NewFCMService(serviceAccountPath string) (*FCMService, error) {
	ctx := context.Background()

	opt := option.WithCredentialsFile(serviceAccountPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("error initializing firebase app: %v", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting messaging client: %v", err)
	}

	log.Println("FCM Service initialized successfully")
	return &FCMService{client: client}, nil
}

// SendYourTurnNotification sends a push notification to a user that it's their turn
func (s *FCMService) SendYourTurnNotification(fcmToken string, timeoutInSec int) error {
	if fcmToken == "" {
		log.Println("FCM token is empty, skipping notification")
		return nil
	}

	message := &messaging.Message{
		Token: fcmToken,
		Notification: &messaging.Notification{
			Title: "🎉 It's Your Turn!",
			Body:  fmt.Sprintf("Please confirm your presence within %d seconds", timeoutInSec),
		},
		Data: map[string]string{
			"type":           "your_turn",
			"timeout_in_sec": fmt.Sprintf("%d", timeoutInSec),
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Sound:    "default",
				Priority: messaging.PriorityHigh,
				ChannelID: "queue_channel",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: "🎉 It's Your Turn!",
						Body:  fmt.Sprintf("Please confirm your presence within %d seconds", timeoutInSec),
					},
					Sound: "default",
					Badge: nil,
				},
			},
		},
	}

	ctx := context.Background()
	response, err := s.client.Send(ctx, message)
	if err != nil {
		log.Printf("Error sending FCM notification: %v", err)
		return err
	}

	log.Printf("Successfully sent FCM notification: %s", response)
	return nil
}

// SendToMultipleTokens sends a notification to multiple FCM tokens
func (s *FCMService) SendToMultipleTokens(tokens []string, title, body string) error {
	if len(tokens) == 0 {
		return nil
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
	}

	ctx := context.Background()
	response, err := s.client.SendEachForMulticast(ctx, message)
	if err != nil {
		return err
	}

	log.Printf("Successfully sent %d messages, %d failures", response.SuccessCount, response.FailureCount)
	return nil
}
