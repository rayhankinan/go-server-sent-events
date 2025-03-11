package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo"
	"github.com/r3labs/sse/v2"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()
	eventTTL := 5 * time.Second
	autostream := true

	e := echo.New()
	defer e.Close()

	server := sse.New()
	server.EventTTL = eventTTL
	server.AutoStream = autostream
	defer server.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	go func() {
		pubsub := rdb.PSubscribe(ctx, "sse:*")
		defer pubsub.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case message := <-pubsub.Channel():
				var streamID string
				if _, err := fmt.Sscanf(message.Channel, "sse:%s", &streamID); err != nil {
					e.Logger.Errorf("Error while parsing the channel: %s, err: %s", message.Channel, err)
					continue
				}

				server.Publish(streamID, &sse.Event{
					Data: []byte(message.Payload),
				})
			}
		}
	}()

	e.GET("/subscribe", func(c echo.Context) error {
		e.Logger.Infof("The client is connected: %v\n", c.RealIP())

		go func() {
			<-c.Request().Context().Done()
			e.Logger.Infof("The client is disconnected: %v\n", c.RealIP())
		}()

		server.ServeHTTP(c.Response(), c.Request())

		return nil
	})

	e.POST("/publish", func(c echo.Context) error {
		var data any
		if err := c.Bind(&data); err != nil {
			return c.JSON(http.StatusInternalServerError, fmt.Sprintf("Error while reading the body: %s", err))
		}

		minifiedJSON, err := json.Marshal(data)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, fmt.Sprintf("Error while minifying the body: %s", err))
		}

		streamID := c.QueryParam("stream")
		if err := rdb.Publish(ctx, fmt.Sprintf("sse:%s", streamID), string(minifiedJSON)).Err(); err != nil {
			return c.JSON(http.StatusInternalServerError, fmt.Sprintf("Error while publishing: %s", err))
		}

		return c.JSON(http.StatusOK, "Published")
	})

	if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		e.Logger.Fatal(err)
	}
}
