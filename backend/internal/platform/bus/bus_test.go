package bus

import (
	"context"
	"testing"
)

func TestPublishDeliversToAllSubscribers(t *testing.T) {
	bus := New()
	var gotA, gotB []NotificationEvent
	bus.Subscribe(func(_ context.Context, event NotificationEvent) { gotA = append(gotA, event) })
	bus.Subscribe(func(_ context.Context, event NotificationEvent) { gotB = append(gotB, event) })

	event := NotificationEvent{RecipientID: 4, ActorID: 7, Kind: "comment", VideoID: 5, CommentID: 9, VideoTitle: "标题", CommentPreview: "预览"}
	bus.Publish(context.Background(), event)

	if len(gotA) != 1 || gotA[0] != event {
		t.Fatalf("subscriber A = %#v", gotA)
	}
	if len(gotB) != 1 || gotB[0] != event {
		t.Fatalf("subscriber B = %#v", gotB)
	}
}

func TestSubscribeAfterPublishOnlyAffectsLaterEvents(t *testing.T) {
	bus := New()
	bus.Publish(context.Background(), NotificationEvent{RecipientID: 1, Kind: "like"})
	late := 0
	bus.Subscribe(func(context.Context, NotificationEvent) { late++ })
	bus.Publish(context.Background(), NotificationEvent{RecipientID: 2, Kind: "like"})
	if late != 1 {
		t.Fatalf("late subscriber deliveries = %d, want 1", late)
	}
}

func TestZeroValueBusIsUsable(t *testing.T) {
	var bus Bus
	bus.Publish(context.Background(), NotificationEvent{})
	bus.Subscribe(func(context.Context, NotificationEvent) {})
	bus.Publish(context.Background(), NotificationEvent{})
}
