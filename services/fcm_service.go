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
		log.Println("[FCM] Token is empty, skipping notification")
		return nil
	}

	log.Printf("[FCM] Sending notification to token: %s", fcmToken[:20]+"...") // Log partial token

	message := &messaging.Message{
		Token: fcmToken,
		Notification: &messaging.Notification{
			Title: "🎉 It's Your Turn!",
			Body:  "Please confirm your presence within 3 minutes",
		},
		Data: map[string]string{
			"type":           "your_turn",
			"timeout_in_sec": fmt.Sprintf("%d", timeoutInSec),
			"click_action":   "FLUTTER_NOTIFICATION_CLICK",
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Sound:           "default",
				Priority:        messaging.PriorityHigh,
				ChannelID:       "queue_channel",
				DefaultSound:    true,
				DefaultVibrateTimings: true,
				Visibility:      messaging.VisibilityPublic,
			},
		},
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority": "10",
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: "🎉 It's Your Turn!",
						Body:  "Please confirm your presence within 3 minutes",
					},
					Sound:            "default",
					Badge:            nil,
					ContentAvailable: true,
				},
			},
		},
	}

	ctx := context.Background()
	response, err := s.client.Send(ctx, message)
	if err != nil {
		log.Printf("[FCM] Error sending notification: %v", err)
		return err
	}

	log.Printf("[FCM] Successfully sent notification. Message ID: %s", response)
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

// SendAdminUserJoinedNotification sends a notification to all admin devices when a user joins the queue
func (s *FCMService) SendAdminUserJoinedNotification(adminTokens []string, username string) error {
	if len(adminTokens) == 0 {
		log.Println("[FCM] No admin tokens available, skipping notification")
		return nil
	}

	log.Printf("[FCM] Sending user joined notification to %d admin(s)", len(adminTokens))

	bodyText := fmt.Sprintf("%s has joined the queue", username)

	message := &messaging.MulticastMessage{
		Tokens: adminTokens,
		Notification: &messaging.Notification{
			Title: "👤 New User Joined",
			Body:  bodyText,
		},
		Data: map[string]string{
			"type":           "user_joined",
			"username":       username,
			"click_action":   "FLUTTER_NOTIFICATION_CLICK",
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Sound:           "default",
				Priority:        messaging.PriorityHigh,
				ChannelID:       "queue_channel",
				DefaultSound:    true,
				DefaultVibrateTimings: true,
				Visibility:      messaging.VisibilityPublic,
			},
		},
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority": "10",
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: "👤 New User Joined",
						Body:  bodyText,
					},
					Sound:            "default",
					Badge:            nil,
					ContentAvailable: true,
				},
			},
		},
	}

	ctx := context.Background()
	response, err := s.client.SendEachForMulticast(ctx, message)
	if err != nil {
		log.Printf("[FCM] Error sending admin notification: %v", err)
		return err
	}

	log.Printf("[FCM] Successfully sent admin notification. Success: %d, Failures: %d", response.SuccessCount, response.FailureCount)
	return nil
}
