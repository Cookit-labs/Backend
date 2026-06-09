package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

type PubSub struct {
	client *redis.Client
	hub    *Hub
	subs   map[string]*redis.PubSub
}

func NewPubSub(redisURL string, hub *Hub) (*PubSub, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*1e9) // 5 seconds
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	ps := &PubSub{
		client: client,
		hub:    hub,
		subs:   make(map[string]*redis.PubSub),
	}

	return ps, nil
}

// Subscribe listens for messages on a channel and forwards to WebSocket hub
func (ps *PubSub) Subscribe(ctx context.Context, channel string) {
	sub := ps.client.Subscribe(ctx, channel)
	ps.subs[channel] = sub

	go func() {
		ch := sub.Channel()
		for msg := range ch {
			// Parse message
			var payload map[string]any
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				log.Printf("failed to unmarshal pubsub message: %v", err)
				continue
			}

			// Extract intent ID and message type
			msgType, ok := payload["type"].(string)
			if !ok {
				continue
			}

			data, ok := payload["payload"]
			if !ok {
				continue
			}

			intentID, ok := payload["intent_id"].(string)
			if !ok {
				continue
			}

			// Broadcast to local WebSocket subscribers
			ps.hub.Broadcast(intentID, msgType, data)
		}
	}()
}

// Publish sends a message to Redis pub/sub
func (ps *PubSub) Publish(ctx context.Context, intentID, msgType string, payload any) error {
	message := map[string]any{
		"type":      msgType,
		"payload":   payload,
		"intent_id": intentID,
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	channel := fmt.Sprintf("intent:%s", intentID)
	if err := ps.client.Publish(ctx, channel, string(data)).Err(); err != nil {
		return fmt.Errorf("redis publish failed: %w", err)
	}

	return nil
}

// PublishLeaderboardUpdate broadcasts leaderboard changes to all clients
func (ps *PubSub) PublishLeaderboardUpdate(ctx context.Context, payload any) error {
	message := map[string]any{
		"type":    "leaderboard_update",
		"payload": payload,
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := ps.client.Publish(ctx, "leaderboard", string(data)).Err(); err != nil {
		return fmt.Errorf("redis publish failed: %w", err)
	}

	return nil
}

// Close closes all subscriptions and the Redis client
func (ps *PubSub) Close() error {
	for _, sub := range ps.subs {
		if err := sub.Close(); err != nil {
			return err
		}
	}
	return ps.client.Close()
}
