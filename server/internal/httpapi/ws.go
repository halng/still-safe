package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"nhooyr.io/websocket"
	"safecircle/internal/store"
	"safecircle/internal/token"
)

func (s *Server) ws(c *gin.Context) {
	raw := bearer(c)
	if raw == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in required."})
		return
	}
	user, err := s.store.UserByToken(c.Request.Context(), token.Hash(raw), s.Now())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in required."})
		return
	}
	if !user.IDOnFile {
		c.JSON(http.StatusForbidden, gin.H{"error": "Add your ID before using Circle."})
		return
	}
	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	conn.SetReadLimit(1024)
	ctx := conn.CloseRead(c.Request.Context())
	s.pump(ctx, conn, user)
}

func (s *Server) pump(ctx context.Context, conn *websocket.Conn, user store.User) {
	sub := s.hub.Add(user.ID)
	defer s.hub.Remove(sub)
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-sub.send:
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
