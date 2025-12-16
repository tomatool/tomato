package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/redis/go-redis/v9"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

type Redis struct {
	name      string
	config    config.Resource
	container *container.Manager
	client    *redis.Client
}

func NewRedis(name string, cfg config.Resource, cm *container.Manager) (*Redis, error) {
	return &Redis{
		name:      name,
		config:    cfg,
		container: cm,
	}, nil
}

func (r *Redis) Name() string { return r.name }

func (r *Redis) Init(ctx context.Context) error {
	host, err := r.container.GetHost(ctx, r.config.Container)
	if err != nil {
		return fmt.Errorf("getting container host: %w", err)
	}
	port, err := r.container.GetPort(ctx, r.config.Container, "6379/tcp")
	if err != nil {
		return fmt.Errorf("getting container port: %w", err)
	}

	db := 0
	if d, ok := r.config.Options["db"].(int); ok {
		db = d
	}
	password := ""
	if p, ok := r.config.Options["password"].(string); ok {
		password = p
	}

	r.client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})
	return nil
}

func (r *Redis) Ready(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *Redis) Reset(ctx context.Context) error {
	strategy := "flush"
	if s, ok := r.config.Options["reset_strategy"].(string); ok {
		strategy = s
	}

	switch strategy {
	case "flush":
		return r.client.FlushDB(ctx).Err()
	case "pattern":
		pattern := "*"
		if p, ok := r.config.Options["reset_pattern"].(string); ok {
			pattern = p
		}
		return r.deleteByPattern(ctx, pattern)
	default:
		return r.client.FlushDB(ctx).Err()
	}
}

func (r *Redis) deleteByPattern(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (r *Redis) RegisterSteps(ctx *godog.ScenarioContext) {
	// Basic operations
	ctx.Step(fmt.Sprintf(`^I set "%s" key "([^"]*)" with value "([^"]*)"$`, r.name), r.setKey)
	ctx.Step(fmt.Sprintf(`^I set "%s" key "([^"]*)" with value "([^"]*)" and TTL "([^"]*)"$`, r.name), r.setKeyWithTTL)
	ctx.Step(fmt.Sprintf(`^I set "%s" key "([^"]*)" with JSON:$`, r.name), r.setKeyJSON)
	ctx.Step(fmt.Sprintf(`^I delete "%s" key "([^"]*)"$`, r.name), r.deleteKey)

	// Assertions
	ctx.Step(fmt.Sprintf(`^"%s" key "([^"]*)" should exist$`, r.name), r.keyShouldExist)
	ctx.Step(fmt.Sprintf(`^"%s" key "([^"]*)" should not exist$`, r.name), r.keyShouldNotExist)
	ctx.Step(fmt.Sprintf(`^"%s" key "([^"]*)" should have value "([^"]*)"$`, r.name), r.keyShouldHaveValue)
	ctx.Step(fmt.Sprintf(`^"%s" key "([^"]*)" should contain "([^"]*)"$`, r.name), r.keyShouldContain)
	ctx.Step(fmt.Sprintf(`^"%s" key "([^"]*)" should have TTL greater than "(\d+)" seconds$`, r.name), r.keyShouldHaveTTL)
	ctx.Step(fmt.Sprintf(`^"%s" should have "(\d+)" keys$`, r.name), r.shouldHaveKeyCount)
	ctx.Step(fmt.Sprintf(`^"%s" should be empty$`, r.name), r.shouldBeEmpty)

	// Hash operations
	ctx.Step(fmt.Sprintf(`^I set "%s" hash "([^"]*)" with fields:$`, r.name), r.setHash)
	ctx.Step(fmt.Sprintf(`^"%s" hash "([^"]*)" field "([^"]*)" should be "([^"]*)"$`, r.name), r.hashFieldShouldBe)
	ctx.Step(fmt.Sprintf(`^"%s" hash "([^"]*)" should contain:$`, r.name), r.hashShouldContain)

	// List operations
	ctx.Step(fmt.Sprintf(`^I push "([^"]*)" to "%s" list "([^"]*)"$`, r.name), r.pushToList)
	ctx.Step(fmt.Sprintf(`^I push values to "%s" list "([^"]*)":$`, r.name), r.pushMultipleToList)
	ctx.Step(fmt.Sprintf(`^"%s" list "([^"]*)" should have "(\d+)" items$`, r.name), r.listShouldHaveLength)
	ctx.Step(fmt.Sprintf(`^"%s" list "([^"]*)" should contain "([^"]*)"$`, r.name), r.listShouldContain)

	// Set operations
	ctx.Step(fmt.Sprintf(`^I add "([^"]*)" to "%s" set "([^"]*)"$`, r.name), r.addToSet)
	ctx.Step(fmt.Sprintf(`^I add members to "%s" set "([^"]*)":$`, r.name), r.addMultipleToSet)
	ctx.Step(fmt.Sprintf(`^"%s" set "([^"]*)" should contain "([^"]*)"$`, r.name), r.setShouldContain)
	ctx.Step(fmt.Sprintf(`^"%s" set "([^"]*)" should have "(\d+)" members$`, r.name), r.setShouldHaveSize)

	// Increment/Decrement
	ctx.Step(fmt.Sprintf(`^I increment "%s" key "([^"]*)"$`, r.name), r.incrementKey)
	ctx.Step(fmt.Sprintf(`^I increment "%s" key "([^"]*)" by "(\d+)"$`, r.name), r.incrementKeyBy)
	ctx.Step(fmt.Sprintf(`^I decrement "%s" key "([^"]*)"$`, r.name), r.decrementKey)
}

func (r *Redis) setKey(key, value string) error {
	return r.client.Set(context.Background(), key, value, 0).Err()
}

func (r *Redis) setKeyWithTTL(key, value, ttl string) error {
	duration, err := time.ParseDuration(ttl)
	if err != nil {
		return fmt.Errorf("invalid TTL duration: %w", err)
	}
	return r.client.Set(context.Background(), key, value, duration).Err()
}

func (r *Redis) setKeyJSON(key string, doc *godog.DocString) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(doc.Content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return r.client.Set(context.Background(), key, doc.Content, 0).Err()
}

func (r *Redis) deleteKey(key string) error {
	return r.client.Del(context.Background(), key).Err()
}

func (r *Redis) keyShouldExist(key string) error {
	exists, err := r.client.Exists(context.Background(), key).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("key %q does not exist", key)
	}
	return nil
}

func (r *Redis) keyShouldNotExist(key string) error {
	exists, err := r.client.Exists(context.Background(), key).Result()
	if err != nil {
		return err
	}
	if exists != 0 {
		return fmt.Errorf("key %q exists but should not", key)
	}
	return nil
}

func (r *Redis) keyShouldHaveValue(key, expected string) error {
	value, err := r.client.Get(context.Background(), key).Result()
	if err != nil {
		return fmt.Errorf("getting key %q: %w", key, err)
	}
	if value != expected {
		return fmt.Errorf("key %q: expected %q, got %q", key, expected, value)
	}
	return nil
}

func (r *Redis) keyShouldContain(key, substr string) error {
	value, err := r.client.Get(context.Background(), key).Result()
	if err != nil {
		return fmt.Errorf("getting key %q: %w", key, err)
	}
	if !strings.Contains(value, substr) {
		return fmt.Errorf("key %q value %q does not contain %q", key, value, substr)
	}
	return nil
}

func (r *Redis) keyShouldHaveTTL(key string, minSeconds int) error {
	ttl, err := r.client.TTL(context.Background(), key).Result()
	if err != nil {
		return fmt.Errorf("getting TTL for key %q: %w", key, err)
	}
	if ttl.Seconds() < float64(minSeconds) {
		return fmt.Errorf("key %q TTL is %v, expected at least %d seconds", key, ttl, minSeconds)
	}
	return nil
}

func (r *Redis) shouldHaveKeyCount(expected int) error {
	count, err := r.client.DBSize(context.Background()).Result()
	if err != nil {
		return err
	}
	if int(count) != expected {
		return fmt.Errorf("expected %d keys, got %d", expected, count)
	}
	return nil
}

func (r *Redis) shouldBeEmpty() error {
	count, err := r.client.DBSize(context.Background()).Result()
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("expected empty database, got %d keys", count)
	}
	return nil
}

func (r *Redis) setHash(hash string, table *godog.Table) error {
	if len(table.Rows) < 2 {
		return fmt.Errorf("table must have headers and at least one data row")
	}
	fields := make(map[string]interface{})
	for _, row := range table.Rows[1:] {
		if len(row.Cells) >= 2 {
			fields[row.Cells[0].Value] = row.Cells[1].Value
		}
	}
	return r.client.HSet(context.Background(), hash, fields).Err()
}

func (r *Redis) hashFieldShouldBe(hash, field, expected string) error {
	value, err := r.client.HGet(context.Background(), hash, field).Result()
	if err != nil {
		return fmt.Errorf("getting hash %q field %q: %w", hash, field, err)
	}
	if value != expected {
		return fmt.Errorf("hash %q field %q: expected %q, got %q", hash, field, expected, value)
	}
	return nil
}

func (r *Redis) hashShouldContain(hash string, table *godog.Table) error {
	if len(table.Rows) < 2 {
		return fmt.Errorf("table must have headers and at least one data row")
	}
	for _, row := range table.Rows[1:] {
		if len(row.Cells) >= 2 {
			if err := r.hashFieldShouldBe(hash, row.Cells[0].Value, row.Cells[1].Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Redis) pushToList(value, list string) error {
	return r.client.RPush(context.Background(), list, value).Err()
}

func (r *Redis) pushMultipleToList(list string, table *godog.Table) error {
	var values []interface{}
	for _, row := range table.Rows {
		if len(row.Cells) > 0 {
			values = append(values, row.Cells[0].Value)
		}
	}
	return r.client.RPush(context.Background(), list, values...).Err()
}

func (r *Redis) listShouldHaveLength(list string, expected int) error {
	length, err := r.client.LLen(context.Background(), list).Result()
	if err != nil {
		return err
	}
	if int(length) != expected {
		return fmt.Errorf("list %q: expected %d items, got %d", list, expected, length)
	}
	return nil
}

func (r *Redis) listShouldContain(list, expected string) error {
	values, err := r.client.LRange(context.Background(), list, 0, -1).Result()
	if err != nil {
		return err
	}
	for _, v := range values {
		if v == expected {
			return nil
		}
	}
	return fmt.Errorf("list %q does not contain %q", list, expected)
}

func (r *Redis) addToSet(member, set string) error {
	return r.client.SAdd(context.Background(), set, member).Err()
}

func (r *Redis) addMultipleToSet(set string, table *godog.Table) error {
	var members []interface{}
	for _, row := range table.Rows {
		if len(row.Cells) > 0 {
			members = append(members, row.Cells[0].Value)
		}
	}
	return r.client.SAdd(context.Background(), set, members...).Err()
}

func (r *Redis) setShouldContain(set, member string) error {
	isMember, err := r.client.SIsMember(context.Background(), set, member).Result()
	if err != nil {
		return err
	}
	if !isMember {
		return fmt.Errorf("set %q does not contain %q", set, member)
	}
	return nil
}

func (r *Redis) setShouldHaveSize(set string, expected int) error {
	size, err := r.client.SCard(context.Background(), set).Result()
	if err != nil {
		return err
	}
	if int(size) != expected {
		return fmt.Errorf("set %q: expected %d members, got %d", set, expected, size)
	}
	return nil
}

func (r *Redis) incrementKey(key string) error {
	return r.client.Incr(context.Background(), key).Err()
}

func (r *Redis) incrementKeyBy(key string, amount int) error {
	return r.client.IncrBy(context.Background(), key, int64(amount)).Err()
}

func (r *Redis) decrementKey(key string) error {
	return r.client.Decr(context.Background(), key).Err()
}

// CacheStore interface implementation
func (r *Redis) Set(ctx context.Context, key, value string) error {
	return r.client.Set(ctx, key, value, 0).Err()
}

func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *Redis) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *Redis) Exists(ctx context.Context, key string) (bool, error) {
	result, err := r.client.Exists(ctx, key).Result()
	return result > 0, err
}

func (r *Redis) Cleanup(ctx context.Context) error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

var _ Handler = (*Redis)(nil)
var _ CacheStore = (*Redis)(nil)
