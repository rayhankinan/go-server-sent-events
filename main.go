package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo"
	"github.com/r3labs/sse/v2"
)

func main() {
	streamID := "time"
	eventTTL := 5 * time.Second
	eventInterval := 1 * time.Second

	e := echo.New()
	defer e.Close()

	server := sse.New()
	server.EventTTL = eventTTL
	defer server.Close()

	server.CreateStream(streamID)

	go func(s *sse.Server, streamID string, eventInterval time.Duration) {
		ticker := time.NewTicker(eventInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.Publish(streamID, &sse.Event{
				Data: fmt.Appendf(nil, "Time: %s", time.Now().Format(time.RFC3339Nano)),
			})
		}
	}(server, streamID, eventInterval)

	e.GET("/sse", func(c echo.Context) error {
		e.Logger.Printf("The client is connected: %v\n", c.RealIP())

		go func() {
			<-c.Request().Context().Done()
			e.Logger.Printf("The client is disconnected: %v\n", c.RealIP())
		}()

		server.ServeHTTP(c.Response(), c.Request())

		return nil
	})

	if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		e.Logger.Fatal(err)
	}
}
