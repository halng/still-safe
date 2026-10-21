package live

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Live struct {
	rdb *redis.Client
}

func Open(url string) (*Live, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return &Live{rdb: rdb}, nil
}

func (l *Live) Ping(ctx context.Context) error {
	if l == nil || l.rdb == nil {
		return errors.New("redis unavailable")
	}
	return l.rdb.Ping(ctx).Err()
}

func (l *Live) Close() error {
	if l == nil || l.rdb == nil {
		return nil
	}
	return l.rdb.Close()
}

type Event struct {
	Type       string  `json:"type"`
	PersonID   string  `json:"personId"`
	Latitude   float64 `json:"latitude,omitempty"`
	Longitude  float64 `json:"longitude,omitempty"`
	AccuracyM  float64 `json:"accuracyM,omitempty"`
	RecordedAt string  `json:"recordedAt,omitempty"`
	Stale      bool    `json:"stale,omitempty"`
	Enabled    *bool   `json:"enabled,omitempty"`
	Check      any     `json:"check,omitempty"`
}

func (l *Live) publish(ctx context.Context, channel string, ev Event) error {
	if l == nil || l.rdb == nil {
		return errors.New("redis unavailable")
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return l.rdb.Publish(ctx, channel, payload).Err()
}

func (l *Live) PublishLocation(ctx context.Context, personID string, lat, lng, accuracy float64, at time.Time) error {
	if l == nil || l.rdb == nil {
		return errors.New("redis unavailable")
	}
	if err := l.rdb.GeoAdd(ctx, "geo:live", &redis.GeoLocation{
		Name:      personID,
		Longitude: lng,
		Latitude:  lat,
	}).Err(); err != nil {
		return err
	}
	return l.publish(ctx, "loc:"+personID, Event{
		Type:       "location",
		PersonID:   personID,
		Latitude:   lat,
		Longitude:  lng,
		AccuracyM:  accuracy,
		RecordedAt: at.UTC().Format(time.RFC3339Nano),
	})
}

func (l *Live) PublishSharing(ctx context.Context, personID string, enabled bool) error {
	return l.publish(ctx, "sharing:"+personID, Event{
		Type:     "sharing",
		PersonID: personID,
		Enabled:  &enabled,
	})
}

func (l *Live) PublishCheck(ctx context.Context, personID string, check any) error {
	return l.publish(ctx, "check:"+personID, Event{
		Type:     "check",
		PersonID: personID,
		Check:    check,
	})
}

func (l *Live) Position(ctx context.Context, personID string) (lng, lat float64, ok bool, err error) {
	if l == nil || l.rdb == nil {
		return 0, 0, false, errors.New("redis unavailable")
	}
	locs, err := l.rdb.GeoPos(ctx, "geo:live", personID).Result()
	if err != nil {
		return 0, 0, false, err
	}
	if len(locs) == 0 || locs[0] == nil {
		return 0, 0, false, nil
	}
	return locs[0].Longitude, locs[0].Latitude, true, nil
}

func (l *Live) Listen(ctx context.Context, on func(payload string)) error {
	if l == nil || l.rdb == nil {
		return errors.New("redis unavailable")
	}
	pubsub := l.rdb.PSubscribe(ctx, "loc:*", "check:*", "sharing:*")
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		return err
	}
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			on(msg.Payload)
		}
	}
}
