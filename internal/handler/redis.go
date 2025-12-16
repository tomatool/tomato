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
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the Redis handler
func (r *Redis) Steps() StepCategory {
	return StepCategory{
		Name:        "Redis",
		Description: "Steps for interacting with Redis key-value store",
		Steps: []StepDef{
			// Basic operations
			{
				Pattern:     `^"{resource}" key "([^"]*)" is "([^"]*)"$`,
				Description: "Sets a string value for a key",
				Example:     `"{resource}" key "user:1" is "John"`,
				Handler:     r.setKey,
			},
			{
				Pattern:     `^"{resource}" key "([^"]*)" is "([^"]*)" with TTL "([^"]*)"$`,
				Description: "Sets a string value with expiration time",
				Example:     `"{resource}" key "session:abc" is "data" with TTL "1h"`,
				Handler:     r.setKeyWithTTL,
			},
			{
				Pattern:     `^"{resource}" key "([^"]*)" is:$`,
				Description: "Sets a JSON value for a key",
				Example:     "\"{resource}\" key \"user:1\" is:\n  \"\"\"\n  {\"name\": \"John\"}\n  \"\"\"",
				Handler:     r.setKeyJSON,
			},
			{
				Pattern:     `^"{resource}" key "([^"]*)" is deleted$`,
				Description: "Deletes a key",
				Example:     `"{resource}" key "user:1" is deleted`,
				Handler:     r.deleteKey,
			},

			// Assertions
			{
				Pattern:     `^"{resource}" key "([^"]*)" exists$`,
				Description: "Asserts that a key exists",
				Example:     `"{resource}" key "user:1" exists`,
				Handler:     r.keyShouldExist,
			},
			{
				Pattern:     `^"{resource}" key "([^"]*)" does not exist$`,
				Description: "Asserts that a key does not exist",
				Example:     `"{resource}" key "user:1" does not exist`,
				Handler:     r.keyShouldNotExist,
			},
			{
				Pattern:     `^"{resource}" key "([^"]*)" has value "([^"]*)"$`,
				Description: "Asserts a key has the exact value",
				Example:     `"{resource}" key "user:1" has value "John"`,
				Handler:     r.keyShouldHaveValue,
			},
			{
				Pattern:     `^"{resource}" key "([^"]*)" contains "([^"]*)"$`,
				Description: "Asserts a key's value contains a substring",
				Example:     `"{resource}" key "user:1" contains "John"`,
				Handler:     r.keyShouldContain,
			},
			{
				Pattern:     `^"{resource}" key "([^"]*)" has TTL greater than "(\d+)" seconds$`,
				Description: "Asserts a key has TTL greater than specified seconds",
				Example:     `"{resource}" key "session:abc" has TTL greater than "3600" seconds`,
				Handler:     r.keyShouldHaveTTL,
			},
			{
				Pattern:     `^"{resource}" has "(\d+)" keys$`,
				Description: "Asserts the database has exactly N keys",
				Example:     `"{resource}" has "5" keys`,
				Handler:     r.shouldHaveKeyCount,
			},
			{
				Pattern:     `^"{resource}" is empty$`,
				Description: "Asserts the database has no keys",
				Example:     `"{resource}" is empty`,
				Handler:     r.shouldBeEmpty,
			},

			// Hash operations
			{
				Pattern:     `^"{resource}" hash "([^"]*)" has fields:$`,
				Description: "Sets multiple fields in a hash",
				Example:     "\"{resource}\" hash \"user:1\" has fields:\n  | field | value |\n  | name  | John  |\n  | age   | 30    |",
				Handler:     r.setHash,
			},
			{
				Pattern:     `^"{resource}" hash "([^"]*)" field "([^"]*)" is "([^"]*)"$`,
				Description: "Asserts a hash field has the expected value",
				Example:     `"{resource}" hash "user:1" field "name" is "John"`,
				Handler:     r.hashFieldShouldBe,
			},
			{
				Pattern:     `^"{resource}" hash "([^"]*)" contains:$`,
				Description: "Asserts a hash contains the specified field-value pairs",
				Example:     "\"{resource}\" hash \"user:1\" contains:\n  | field | value |\n  | name  | John  |",
				Handler:     r.hashShouldContain,
			},

			// List operations
			{
				Pattern:     `^"{resource}" list "([^"]*)" has "([^"]*)"$`,
				Description: "Pushes a value to the end of a list",
				Example:     `"{resource}" list "queue" has "item1"`,
				Handler:     r.pushToList,
			},
			{
				Pattern:     `^"{resource}" list "([^"]*)" has values:$`,
				Description: "Pushes multiple values to a list",
				Example:     "\"{resource}\" list \"queue\" has values:\n  | item1 |\n  | item2 |",
				Handler:     r.pushMultipleToList,
			},
			{
				Pattern:     `^"{resource}" list "([^"]*)" has "(\d+)" items$`,
				Description: "Asserts a list has exactly N items",
				Example:     `"{resource}" list "queue" has "3" items`,
				Handler:     r.listShouldHaveLength,
			},
			{
				Pattern:     `^"{resource}" list "([^"]*)" contains "([^"]*)"$`,
				Description: "Asserts a list contains a value",
				Example:     `"{resource}" list "queue" contains "item1"`,
				Handler:     r.listShouldContain,
			},

			// Set operations
			{
				Pattern:     `^"{resource}" set "([^"]*)" has "([^"]*)"$`,
				Description: "Adds a member to a set",
				Example:     `"{resource}" set "tags" has "tag1"`,
				Handler:     r.addToSet,
			},
			{
				Pattern:     `^"{resource}" set "([^"]*)" has members:$`,
				Description: "Adds multiple members to a set",
				Example:     "\"{resource}\" set \"tags\" has members:\n  | tag1 |\n  | tag2 |",
				Handler:     r.addMultipleToSet,
			},
			{
				Pattern:     `^"{resource}" set "([^"]*)" contains "([^"]*)"$`,
				Description: "Asserts a set contains a member",
				Example:     `"{resource}" set "tags" contains "tag1"`,
				Handler:     r.setShouldContain,
			},
			{
				Pattern:     `^"{resource}" set "([^"]*)" has "(\d+)" members$`,
				Description: "Asserts a set has exactly N members",
				Example:     `"{resource}" set "tags" has "3" members`,
				Handler:     r.setShouldHaveSize,
			},

			// Increment/Decrement
			{
				Pattern:     `^"{resource}" key "([^"]*)" is incremented$`,
				Description: "Increments a key's integer value by 1",
				Example:     `"{resource}" key "counter" is incremented`,
				Handler:     r.incrementKey,
			},
			{
				Pattern:     `^"{resource}" key "([^"]*)" is incremented by "(\d+)"$`,
				Description: "Increments a key's integer value by N",
				Example:     `"{resource}" key "counter" is incremented by "5"`,
				Handler:     r.incrementKeyBy,
			},
			{
				Pattern:     `^"{resource}" key "([^"]*)" is decremented$`,
				Description: "Decrements a key's integer value by 1",
				Example:     `"{resource}" key "counter" is decremented`,
				Handler:     r.decrementKey,
			},
		},
	}
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

func (r *Redis) pushToList(list, value string) error {
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

func (r *Redis) addToSet(set, member string) error {
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
