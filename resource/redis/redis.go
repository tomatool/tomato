package redis

import (
	"errors"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/tomatool/tomato/config"
)

// Redis contains the configuration for the redis resource
type Redis struct {
	pool       *redis.Pool
	datasource string
}

func New(cfg *config.Resource) (*Redis, error) {
	u, ok := cfg.Options["datasource"]
	if !ok {
		return nil, errors.New("redis: datasource is required")
	}
	return &Redis{datasource: u}, nil
}

func (r *Redis) Open() error {
	r.pool = &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.DialURL(r.datasource)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}
	return nil
}

func (r *Redis) Ready() error {
	conn := r.pool.Get()
	defer conn.Close()
	_, err := conn.Do("PING")
	return err
}

func (r *Redis) Reset() error {
	conn := r.pool.Get()
	defer conn.Close()
	_, err := conn.Do("FLUSHDB")
	return err
}

func (r *Redis) Close() error {
	return r.pool.Close()
}

func (r *Redis) Set(key string, value string) error {
	conn := r.pool.Get()
	defer conn.Close()
	_, err := conn.Do("SET", key, value)
	return err
}

func (r *Redis) Get(key string) (string, error) {
	conn := r.pool.Get()
	defer conn.Close()
	reply, err := conn.Do("GET", key)
	if err != nil {
		return "", err
	}
	if result, err := redis.String(reply, nil); err == nil {
		return result, err
	}
	return fmt.Sprint(reply), nil
}

func (r *Redis) Exists(key string) (bool, error) {
	conn := r.pool.Get()
	defer conn.Close()
	return redis.Bool(conn.Do("EXISTS", key))
}
