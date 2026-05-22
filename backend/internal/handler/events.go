package handler

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/scheduler"
)

type EventsHandler struct {
	eventBus *scheduler.EventBus
}

func NewEventsHandler(eventBus *scheduler.EventBus) *EventsHandler {
	return &EventsHandler{eventBus: eventBus}
}

func (h *EventsHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/tasks/events", h.streamEvents)
}

func (h *EventsHandler) streamEvents(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	ch := h.eventBus.Subscribe()
	defer h.eventBus.Unsubscribe(ch)

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
		}
	}
}
