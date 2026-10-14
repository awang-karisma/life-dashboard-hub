package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
)

type EventsHandler struct {
	Bus *IngestionBus
}

func NewEventsHandler(bus *IngestionBus) *EventsHandler {
	return &EventsHandler{
		Bus: bus,
	}
}

func (h *EventsHandler) StreamIngestion(c fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	userID, _ := c.Locals("userID").(uint)

	subID, eventCh := h.Bus.Subscribe()
	recentEvents := h.Bus.Recent()
	ctx := c.Context()

	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer h.Bus.Unsubscribe(subID)

		// Send recent events for initial hydration
		for _, ev := range recentEvents {
			if ev.UserID != userID {
				continue
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
		}
		if err := w.Flush(); err != nil {
			return
		}

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-eventCh:
				if !ok {
					return
				}
				if ev.UserID != userID {
					continue
				}
				data, err := json.Marshal(ev)
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			case <-ticker.C:
				if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})
}
