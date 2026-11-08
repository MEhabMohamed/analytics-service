// Package repository implements MongoDB data access for analytics-service.
//
// TS vs Go Syntax Note (Implicit Interfaces / Duck Typing):
//   - Notice that `EventIDsRepo` never writes `implements coreevents.EventDeduper`—and doesn't
//     even import `lib/coreevents`!
//   - In Go, interface satisfaction is implicit ("structural typing"): because `*EventIDsRepo`
//     has a method `MarkSeen(ctx context.Context, eventID string, eventType string) (bool, error)`,
//     it automatically satisfies `coreevents.EventDeduper` when wired in `lib/boot/boot.go`.
package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"quickbite/analytics-service/app/analytics"
	"quickbite/analytics-service/app/analytics/entity"
)

// EventIDsRepo manages the event_ids collection and implicitly satisfies coreevents.EventDeduper.
type EventIDsRepo struct {
	coll *mongo.Collection
}

// NewEventIDsRepo constructs an EventIDsRepo backed by the event_ids collection.
func NewEventIDsRepo(db *mongo.Database) *EventIDsRepo {
	return &EventIDsRepo{
		coll: db.Collection(analytics.CollEventIDs),
	}
}

// MarkSeen inserts eventID into event_ids.
// Returns (true, nil) if inserted for the first time, or (false, nil) if a duplicate key error occurred.
func (r *EventIDsRepo) MarkSeen(ctx context.Context, eventID string, eventType string) (bool, error) {
	doc := entity.EventIDDoc{
		EventID:    eventID,
		EventType:  eventType,
		ReceivedAt: time.Now().UTC(),
	}
	_, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// FilterAndMarkBatchSeen checks which eventIDs in a batch have not been seen yet using a single
// `$in` query and inserts all unseen eventIDs in a single unordered BulkWrite (2 round-trips total
// instead of N per-row InsertOne calls inside a for loop).
// Returns a map containing `true` for every newly-inserted (fresh) eventID.
func (r *EventIDsRepo) FilterAndMarkBatchSeen(ctx context.Context, eventIDs []string, eventType string) (map[string]bool, error) {
	fresh := make(map[string]bool, len(eventIDs))
	if len(eventIDs) == 0 {
		return fresh, nil
	}

	cur, err := r.coll.Find(ctx, bson.M{"event_id": bson.M{"$in": eventIDs}})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cur.Close(ctx) }()

	var existing []entity.EventIDDoc
	if err := cur.All(ctx, &existing); err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(existing))
	for _, doc := range existing {
		seen[doc.EventID] = true
	}

	now := time.Now().UTC()
	models := make([]mongo.WriteModel, 0, len(eventIDs))
	for _, id := range eventIDs {
		if seen[id] || fresh[id] {
			continue
		}
		fresh[id] = true
		models = append(models, mongo.NewInsertOneModel().SetDocument(entity.EventIDDoc{
			EventID:    id,
			EventType:  eventType,
			ReceivedAt: now,
		}))
	}

	if len(models) > 0 {
		_, err := r.coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			return nil, err
		}
	}

	return fresh, nil
}

