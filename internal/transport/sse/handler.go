package sse

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type BrokerInterface interface {
	Subscribe() chan []byte
	Unsubscribe(chan []byte)
}

type Handler struct {
	broker BrokerInterface
}

func NewHandler(broker BrokerInterface) *Handler {
	return &Handler{broker: broker}
}

func (h *Handler) StreamHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Transfer-Encoding", "chunked")

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.Error(fmt.Errorf("streaming not supported"))
			return
		}

		flusher.Flush()

		sub := h.broker.Subscribe()
		defer h.broker.Unsubscribe(sub)

		for {
			select {
			case msg, ok := <-sub:
				if !ok {
					return
				}
				_, err := fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
				if err != nil {
					return
				}
				flusher.Flush()
			case <-c.Request.Context().Done():
				return
			}
		}
	}
}