package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"safecircle/internal/live"
	"safecircle/internal/store"
	"safecircle/internal/token"
)

const (
	tokenTTL   = 30 * 24 * time.Hour
	staleAfter = 15 * time.Minute
)

type Server struct {
	store      *store.Store
	live       *live.Live
	hub        *Hub
	uploadDir  string
	bcryptCost int
	now        func() time.Time
	quiet      bool
}

type Options struct {
	Store      *store.Store
	Live       *live.Live
	UploadDir  string
	BcryptCost int
	Now        func() time.Time
	Quiet      bool
}

func New(opt Options) *Server {
	now := opt.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	cost := opt.BcryptCost
	if cost == 0 {
		cost = 12
	}
	return &Server{
		store:      opt.Store,
		live:       opt.Live,
		hub:        NewHub(),
		uploadDir:  opt.UploadDir,
		bcryptCost: cost,
		now:        now,
		quiet:      opt.Quiet,
	}
}

func (s *Server) Now() time.Time { return s.now().UTC() }

func (s *Server) Handler() http.Handler {
	if s.quiet {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	if !s.quiet {
		r.Use(gin.Logger())
	}
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	r.GET("/healthz", s.health)
	v := r.Group("/v1")
	v.POST("/auth/register", s.register)
	v.POST("/auth/login", s.login)
	v.GET("/ws", s.ws)

	auth := v.Group("")
	auth.Use(s.requireUser)
	auth.POST("/auth/logout", s.logout)
	auth.GET("/me", s.me)
	auth.POST("/me/id", s.submitID)

	gated := auth.Group("")
	gated.Use(s.requireID)
	gated.PUT("/circle/limit", s.setLimit)
	gated.POST("/circle/invites", s.createInvite)
	gated.GET("/circle/supervisors", s.listSupervisors)
	gated.DELETE("/circle/supervisors/:linkId", s.revoke)
	gated.POST("/invites/accept", s.acceptInvite)
	gated.PUT("/circle/sharing", s.setSharing)
	gated.POST("/location", s.postLocation)
	gated.GET("/places", s.listPlaces)
	gated.POST("/places", s.createPlace)
	gated.DELETE("/places/:placeId", s.deletePlace)
	gated.GET("/watching", s.watching)
	gated.GET("/people/:personId", s.person)
	gated.GET("/people/:personId/trail", s.trail)
	gated.GET("/people/:personId/places", s.personPlaces)
	gated.POST("/people/:personId/checks", s.startCheck)
	gated.GET("/checks/pending", s.pendingCheck)
	gated.POST("/checks/:checkId/confirm", s.confirmCheck)
	return r
}

func (s *Server) health(c *gin.Context) {
	redisStatus := "down"
	if s.live != nil && s.live.Ping(c.Request.Context()) == nil {
		redisStatus = "up"
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "postgres": "up", "redis": redisStatus})
}

func (s *Server) requireUser(c *gin.Context) {
	raw := bearer(c)
	if raw == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Sign in required."})
		return
	}
	user, err := s.store.UserByToken(c.Request.Context(), token.Hash(raw), s.Now())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Sign in required."})
		return
	}
	c.Set("user", user)
	c.Next()
}

func (s *Server) requireID(c *gin.Context) {
	user := currentUser(c)
	if !user.IDOnFile {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Add your ID before using Circle."})
		return
	}
	c.Next()
}

func currentUser(c *gin.Context) store.User {
	user, _ := c.Get("user")
	return user.(store.User)
}

func bearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return strings.TrimSpace(c.Query("access_token"))
}

func (s *Server) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found."})
	case errors.Is(err, store.ErrCap):
		c.JSON(http.StatusConflict, gin.H{"error": "This circle is full."})
	case errors.Is(err, store.ErrLimit):
		c.JSON(http.StatusConflict, gin.H{"error": "Revoke a supervisor before lowering the limit."})
	case errors.Is(err, store.ErrPending):
		c.JSON(http.StatusConflict, gin.H{"error": "A safety check is already open."})
	case errors.Is(err, store.ErrWindow):
		c.JSON(http.StatusConflict, gin.H{"error": "The response window has passed."})
	case errors.Is(err, store.ErrSharing):
		c.JSON(http.StatusConflict, gin.H{"error": "Sharing is off."})
	case errors.Is(err, store.ErrSelf):
		c.JSON(http.StatusConflict, gin.H{"error": "You cannot supervise yourself."})
	case errors.Is(err, store.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "That is already in use."})
	default:
		log.Printf("request error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Something went wrong."})
	}
}

type locJSON struct {
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	AccuracyM  float64   `json:"accuracyM"`
	RecordedAt time.Time `json:"recordedAt"`
	Stale      bool      `json:"stale"`
}

func (s *Server) locJSON(loc store.Location) locJSON {
	return locJSON{
		Latitude:   loc.Latitude,
		Longitude:  loc.Longitude,
		AccuracyM:  loc.AccuracyM,
		RecordedAt: loc.RecordedAt,
		Stale:      s.Now().Sub(loc.RecordedAt) > staleAfter,
	}
}

type placeJSON struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func placesJSON(places []store.Place) []placeJSON {
	out := make([]placeJSON, 0, len(places))
	for _, p := range places {
		out = append(out, placeJSON{ID: p.ID, Label: p.Label, Latitude: p.Latitude, Longitude: p.Longitude})
	}
	return out
}

type checkJSON struct {
	ID              string     `json:"id"`
	PersonID        string     `json:"personId"`
	PersonName      string     `json:"personName"`
	StartedBy       string     `json:"startedBy"`
	StartedByName   string     `json:"startedByName"`
	Reason          string     `json:"reason"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"startedAt"`
	RespondBy       time.Time  `json:"respondBy"`
	ResolvedAt      *time.Time `json:"resolvedAt"`
	ConfirmedByName string     `json:"confirmedByName,omitempty"`
}

func toCheckJSON(c store.Check) checkJSON {
	return checkJSON{
		ID:              c.ID,
		PersonID:        c.PersonID,
		PersonName:      c.PersonName,
		StartedBy:       c.StartedBy,
		StartedByName:   c.StartedByName,
		Reason:          c.Reason,
		Status:          c.Status,
		StartedAt:       c.StartedAt,
		RespondBy:       c.RespondBy,
		ResolvedAt:      c.ResolvedAt,
		ConfirmedByName: c.ConfirmedByName,
	}
}

func (s *Server) publishCheck(ctx context.Context, chk store.Check) {
	if s.live == nil {
		return
	}
	if err := s.live.PublishCheck(ctx, chk.PersonID, toCheckJSON(chk)); err != nil {
		log.Printf("redis check: %v", err)
	}
}

func (s *Server) Expire(ctx context.Context) {
	s.expireAndPublish(ctx)
}

func (s *Server) expireAndPublish(ctx context.Context) {
	checks, err := s.store.ExpireChecks(ctx, s.Now())
	if err != nil {
		log.Printf("expire checks: %v", err)
		return
	}
	for _, chk := range checks {
		s.publishCheck(ctx, chk)
	}
}

func (s *Server) Dispatch(ctx context.Context, payload []byte, typ, personID string) {
	s.hub.Dispatch(ctx, payload, func(ctx context.Context, userID string) bool {
		return s.canReceive(ctx, userID, typ, personID)
	})
}

func (s *Server) canReceive(ctx context.Context, userID, typ, personID string) bool {
	if userID == "" || personID == "" {
		return false
	}
	if personID == userID {
		return typ == "location" || typ == "sharing" || typ == "check"
	}
	ok, err := s.store.IsActiveSupervisor(ctx, personID, userID)
	return err == nil && ok
}

func (s *Server) Listen(ctx context.Context) {
	if s.live == nil {
		return
	}
	err := s.live.Listen(ctx, func(payload string) {
		var meta struct {
			Type     string `json:"type"`
			PersonID string `json:"personId"`
		}
		if err := jsonUnmarshal([]byte(payload), &meta); err != nil {
			return
		}
		s.Dispatch(ctx, []byte(payload), meta.Type, meta.PersonID)
	})
	if err != nil && ctx.Err() == nil {
		log.Printf("redis listen: %v", err)
	}
}
