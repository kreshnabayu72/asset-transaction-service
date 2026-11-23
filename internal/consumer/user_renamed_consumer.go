// Package consumer applies events published by other services to asset-transaction-service tables.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	iamevents "gitea.tagsamurai.local/BETS-V2/service-contracts/services/iam/events"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/segmentio/kafka-go"
)

// consumerGroup is shared by every instance so a topic's partitions are split between them.
const consumerGroup = "asset-transaction-iam-user-renamed"

// Postgres codes for a client whose tenant schema or tables are not provisioned here.
const (
	undefinedTable = "42P01"
	invalidSchema  = "3F000"
)

// ModifiedByNameUpdater is the consumer's write seam; TransactionSettingsRepository implements it.
type ModifiedByNameUpdater interface {
	UpdateModifiedByName(ctx context.Context, svc coreService.PostgreSQLServicer, userID int64, name string) error
	UpdateRuleFieldsModifiedByName(ctx context.Context, svc coreService.PostgreSQLServicer, userID int64, name string) error
	UpdateRuleOverridesCreatedByName(ctx context.Context, svc coreService.PostgreSQLServicer, userID int64, name string) error
}

// UserRenamedConsumer keeps denormalized user names on transaction_settings and the rule tables current when iam renames a user.
type UserRenamedConsumer struct {
	dbName  string
	updater ModifiedByNameUpdater
	reader  *kafka.Reader
}

// NewUserRenamedConsumer builds a reader on the iam.user.renamed topic. brokers is comma-separated.
func NewUserRenamedConsumer(dbName string, updater ModifiedByNameUpdater, brokers string) *UserRenamedConsumer {
	return &UserRenamedConsumer{
		dbName:  dbName,
		updater: updater,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     strings.Split(brokers, ","),
			Topic:       (&iamevents.UserRenamed{}).Topic(),
			GroupID:     consumerGroup,
			StartOffset: kafka.LastOffset,
		}),
	}
}

// Run reads until ctx is cancelled. At-least-once: the offset is committed only after the
// update succeeds, so a database failure is redelivered. An undecodable payload or an
// unprovisioned tenant can never succeed, so those are committed and dropped.
func (c *UserRenamedConsumer) Run(ctx context.Context) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return
			}
			slog.ErrorContext(ctx, "user-renamed consumer: fetch", "err", err)
			time.Sleep(time.Second)
			continue
		}

		if err := c.process(ctx, msg.Value); err != nil {
			slog.ErrorContext(ctx, "user-renamed consumer: process (will retry)", "offset", msg.Offset, "err", err)
			time.Sleep(time.Second)
			continue
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			slog.ErrorContext(ctx, "user-renamed consumer: commit", "err", err)
		}
	}
}

// Close releases the reader.
func (c *UserRenamedConsumer) Close() error { return c.reader.Close() }

func (c *UserRenamedConsumer) process(ctx context.Context, payload []byte) error {
	var evt iamevents.UserRenamed
	if err := json.Unmarshal(payload, &evt); err != nil {
		slog.ErrorContext(ctx, "user-renamed consumer: undecodable payload, dropping", "err", err)
		return nil
	}
	userID, err := strconv.ParseInt(evt.UserID, 10, 64)
	if err != nil || evt.ClientID == "" {
		slog.ErrorContext(ctx, "user-renamed consumer: bad event, dropping", "outboxId", evt.OutboxID)
		return nil
	}

	svc := coreService.MakePostgreSQLService(evt.ClientID + ":" + c.dbName)
	_, err = coreService.UseTransactions(ctx, svc.GetPool(), func(tx pgx.Tx) (any, error) {
		svc.SetTransaction(tx)
		defer svc.SetTransaction(nil)
		return nil, c.applyRename(ctx, svc, userID, evt.FullName)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == undefinedTable || pgErr.Code == invalidSchema) {
		slog.WarnContext(ctx, "user-renamed consumer: tenant not provisioned, dropping", "clientId", evt.ClientID)
		return nil
	}
	return err
}

func (c *UserRenamedConsumer) applyRename(ctx context.Context, svc coreService.PostgreSQLServicer, userID int64, name string) error {
	if err := c.updater.UpdateModifiedByName(ctx, svc, userID, name); err != nil {
		return err
	}
	if err := c.updater.UpdateRuleFieldsModifiedByName(ctx, svc, userID, name); err != nil {
		return err
	}
	return c.updater.UpdateRuleOverridesCreatedByName(ctx, svc, userID, name)
}
